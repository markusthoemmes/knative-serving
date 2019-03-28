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
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/knative/pkg/logging"
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

func NewMetricCollector(logger *zap.SugaredLogger, statsScraperFactory StatsScraperFactory) MetricCollector {
	return &collector{
		logger:              logger,
		statsScraperFactory: statsScraperFactory,
		collections:         make(map[string]*collection),
	}
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

type collection struct {
	logger *zap.SugaredLogger
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
				ctx := logging.WithLogger(context.Background(), logger)
				statsCh := make(chan *StatMessage, 1)
				scraper.Scrape(ctx, statsCh)
				message := <-statsCh
				collection.recordStat(message.Stat)
			}
		}
	}()

	return collection
}

func (c *collection) recordStat(stat Stat) {
	c.logger.Infof("GOT STAT %v", stat)
}

func (c *collection) Close() {
	// shutdown scraper
}
