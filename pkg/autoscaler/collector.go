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
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type MetricClient interface {
	GetStableConcurrency(namespace, name string) (float64, error)
	GetPanicConcurrency(namespace, name string) (float64, error)
}

type MetricCollector interface {
	StartCollecting(metric *Metric) error
	UpdateMetadata(metric *Metric)
	StopCollecting(metric *Metric)
	Shutdown()

	MetricClient
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

	scraper, err := c.statsScraperFactory(metric)
	if err != nil {
		return err
	}

	c.collections[key] = newCollection(scraper, c.logger)
	return nil
}

func (c *collector) UpdateMetadata(metric *Metric) {
	// Update window sizes for example.
}

func (c *collector) StopCollecting(metric *Metric) {
	c.collectionsMutex.Lock()
	defer c.collectionsMutex.Unlock()

	c.logger.Infof("Stopping metric collection of %s/%s", metric.Namespace, metric.Name)

	key := NewMetricKey(metric.Namespace, metric.Name)
	if collection, ok := c.collections[key]; ok {
		collection.shutdown()
		delete(c.collections, key)
	}
}

func (c *collector) Shutdown() {
	c.collectionsMutex.Lock()
	defer c.collectionsMutex.Unlock()

	for _, collection := range c.collections {
		collection.shutdown()
	}
	c.collections = nil
}

func (c *collector) GetStableConcurrency(namespace, name string) (float64, error) {
	c.collectionsMutex.RLock()
	defer c.collectionsMutex.RUnlock()

	key := NewMetricKey(namespace, name)
	collection, ok := c.collections[key]
	if !ok {
		return 0.0, fmt.Errorf("no metrics available")
	}

	stable, _ := collection.calculateConcurrencies(time.Now())
	return stable, nil
}

func (c *collector) GetPanicConcurrency(namespace, name string) (float64, error) {
	c.collectionsMutex.RLock()
	defer c.collectionsMutex.RUnlock()

	key := NewMetricKey(namespace, name)
	collection, ok := c.collections[key]
	if !ok {
		return 0.0, fmt.Errorf("no metrics available")
	}

	_, panic := collection.calculateConcurrencies(time.Now())
	return panic, nil
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

	stopCh chan struct{}
	grp    sync.WaitGroup

	buckets     map[time.Time]statsBucket
	bucketMutex sync.RWMutex

	stableWindow time.Duration
	panicWindow  time.Duration
}

func newCollection(scraper StatsScraper, logger *zap.SugaredLogger) *collection {
	c := &collection{
		logger:       logger,
		stopCh:       make(chan struct{}),
		buckets:      make(map[time.Time]statsBucket),
		stableWindow: 60 * time.Second,
		panicWindow:  6 * time.Second,
	}

	c.grp.Add(1)
	go func() {
		defer c.grp.Done()

		scrapeTicker := time.NewTicker(scrapeTickInterval)
		for {
			select {
			case <-c.stopCh:
				return
			case <-scrapeTicker.C:
				message, err := scraper.Scrape()
				if err != nil {
					logger.Errorw("Failed to scrape metrics", zap.Error(err))
				}
				if message != nil {
					c.recordStat(message.Stat)
				}
			}
		}
	}()

	return c
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

func (c *collection) calculateConcurrencies(now time.Time) (stableConcurrency float64, panicConcurrency float64) {
	c.bucketMutex.Lock()
	defer c.bucketMutex.Unlock()

	var (
		stableBuckets float64
		stableTotal   float64

		panicBuckets float64
		panicTotal   float64
	)
	for bucketTime, bucket := range c.buckets {
		if !bucketTime.Add(c.panicWindow).Before(now) {
			panicBuckets++
			panicTotal += bucket.concurrency()
		}

		if !bucketTime.Add(c.stableWindow).Before(now) {
			stableBuckets++
			stableTotal += bucket.concurrency()
		} else {
			delete(c.buckets, bucketTime)
		}
	}

	if stableBuckets > 0 {
		stableConcurrency = stableTotal / stableBuckets
	}
	if panicBuckets > 0 {
		panicConcurrency = panicTotal / panicBuckets
	}

	return
}

func (c *collection) shutdown() {
	close(c.stopCh)
	c.grp.Wait()
}

// Stat defines a single measurement at a point in time
type Stat struct {
	// The time the data point was collected on the pod.
	Time *time.Time

	// The unique identity of this pod.  Used to count how many pods
	// are contributing to the metrics.
	PodName string

	// Average number of requests currently being handled by this pod.
	AverageConcurrentRequests float64

	// Part of AverageConcurrentRequests, for requests going through a proxy.
	AverageProxiedConcurrentRequests float64

	// Number of requests received since last Stat (approximately QPS).
	RequestCount int32

	// Part of RequestCount, for requests going through a proxy.
	ProxiedRequestCount int32
}

// StatMessage wraps a Stat with identifying information so it can be routed
// to the correct receiver.
type StatMessage struct {
	Key  string
	Stat Stat
}

// statsBucket keeps all the stats that fall into a defined bucket.
type statsBucket map[string][]*Stat

// add adds a Stat to the bucket. Stats from the same pod will be
// collapsed.
func (b statsBucket) add(stat *Stat) {
	b[stat.PodName] = append(b[stat.PodName], stat)
}

// concurrency calculates the overall concurrency as measured by this
// bucket. All stats that belong to the same pod will be averaged.
// The overall concurrency is the sum of the measured concurrency of all
// pods (including activator metrics).
func (b statsBucket) concurrency() float64 {
	var total float64
	for _, podStats := range b {
		var subtotal float64
		for _, stat := range podStats {
			// Proxied requests have been counted at the activator. Subtract
			// AverageProxiedConcurrentRequests to avoid double counting.
			subtotal += stat.AverageConcurrentRequests - stat.AverageProxiedConcurrentRequests
		}
		total += subtotal / float64(len(podStats))
	}

	return total
}
