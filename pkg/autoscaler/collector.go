/*
Copyright 2019 The Knative Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package autoscaler

import (
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

type MetricClient interface {
	Get(namespace, name string) (int64, error)
	Watch(namespace, name string, watchFn func(int64))
}

type MetricCollector interface {
	StartCollecting(metric *Metric) error
	StopCollecting(metric *Metric)
	Shutdown()
}

func NewMetricCollector(logger *zap.SugaredLogger, statsScraperFactory StatsScraperFactory, statsCh chan *StatMessage) MetricCollector {
	collector := &collector{
		logger:              logger,
		statsScraperFactory: statsScraperFactory,
		collections:         make(map[string]*collection),
	}

	go func() {
		for {
			select {
			case msg := <-statsCh:
				collector.dispatchStat(msg)
			}
		}
	}()

	return collector
}

type collector struct {
	logger *zap.SugaredLogger

	statsScraperFactory StatsScraperFactory

	collections      map[string]*collection
	collectionsMutex sync.RWMutex
}

func (c *collector) StartCollecting(metric *Metric) error {
	c.collectionsMutex.Lock()
	defer c.collectionsMutex.Unlock()

	c.logger.Infof("Starting metric collection of %s/%s", metric.Namespace, metric.Name)

	key := NewMetricKey(metric.Namespace, metric.Name)
	if _, ok := c.collections[key]; ok {
		return errors.New("collection already exists")
	}

	scraper, err := c.statsScraperFactory(metric, &DynamicConfig{})
	if err != nil {
		return err
	}

	c.collections[key] = newCollection(scraper, c.logger)
	return nil
}

func (c *collector) StopCollecting(metric *Metric) {
	c.collectionsMutex.Lock()
	defer c.collectionsMutex.Unlock()

	c.logger.Infof("Stopping metric collection of %s/%s", metric.Namespace, metric.Name)

	key := NewMetricKey(metric.Namespace, metric.Name)
	if collection, ok := c.collections[key]; ok {
		collection.Close()
		delete(c.collections, key)
	}
}

func (c *collector) Shutdown() {
	c.collectionsMutex.Lock()
	defer c.collectionsMutex.Unlock()

	for _, collection := range c.collections {
		collection.Close()
	}
	c.collections = nil
}

func (c *collector) dispatchStat(msg *StatMessage) {
	c.collectionsMutex.RLock()
	defer c.collectionsMutex.RUnlock()

	if collection, ok := c.collections[msg.Key]; ok {
		collection.recordStat(msg.Stat)
	}
}

type collection struct {
	logger *zap.SugaredLogger

	buckets     map[time.Time]statsBucket
	bucketMutex sync.RWMutex
}

func newCollection(scraper StatsScraper, logger *zap.SugaredLogger) *collection {
	collection := &collection{
		logger: logger,
	}

	go func() {
		scrapeTicker := time.NewTicker(scrapeTickInterval)
		for {
			select {
			case <-scrapeTicker.C:
				message, err := scraper.Scrape()
				if err != nil {
					logger.Errorw("Failed to scrape metrics", zap.Error(err))
				}
				if message != nil {
					collection.recordStat(message.Stat)
				}
			}
		}
	}()

	return collection
}

func (c *collection) recordStat(stat Stat) {
	c.bucketMutex.Lock()
	defer c.bucketMutex.Unlock()

	bucketKey := stat.Time.Truncate(bucketSize)
	bucket, ok := c.buckets[bucketKey]
	if !ok {
		bucket = statsBucket{}
		c.buckets[bucketKey] = bucket
	}
	bucket.add(&stat)
}

func (c *collection) Close() {
	// shutdown scraper
}
