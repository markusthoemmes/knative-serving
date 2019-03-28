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

import "go.uber.org/zap"

type MetricClient interface {
	Get(namespace, name string) (int64, error)
	Watch(namespace, name string, watchFn func(int64))
}

type MetricCollector interface {
	StartCollecting(namespace, name string)
	StopCollecting(namespace, name string)
	Shutdown()
}

func NewMetricCollector(logger *zap.SugaredLogger) MetricCollector {
	return &collector{logger}
}

type collector struct {
	logger *zap.SugaredLogger
}

func (c *collector) StartCollecting(namespace, name string) {
	c.logger.Infof("Starting metric collection of %s/%s", namespace, name)
}

func (c *collector) StopCollecting(namespace, name string) {
	c.logger.Infof("Stopping metric collection of %s/%s", namespace, name)
	// Stop/Delete scraper
}

func (c *collector) Shutdown() {

}

type collection struct {
}

func newCollection() *collection {
	// setup scraper
	// watch for websocket metrics
	return &collection{}
}

func (c *collection) Close() {
	// shutdown scraper
}
