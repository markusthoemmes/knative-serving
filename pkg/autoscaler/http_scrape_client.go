/*
Copyright 2019 The Knative Authors

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
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type httpScrapeClient struct {
	httpClient *http.Client
}

func newHTTPScrapeClient(httpClient *http.Client) (*httpScrapeClient, error) {
	if httpClient == nil {
		return nil, errors.New("HTTP client must not be nil")
	}

	return &httpScrapeClient{
		httpClient: httpClient,
	}, nil
}

func (c *httpScrapeClient) Scrape(url string) (Stat, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Stat{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Stat{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Stat{}, fmt.Errorf("GET request for URL %q returned HTTP status %v", url, resp.StatusCode)
	}

	return extractData(resp.Body)
}

func extractData(body io.Reader) (Stat, error) {
	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(body)
	if err != nil {
		return Stat{}, fmt.Errorf("reading text format failed: %v", err)
	}

	stat := Stat{}
	for m, pv := range map[string]*float64{
		"queue_average_concurrent_requests":         &stat.AverageConcurrentRequests,
		"queue_average_proxied_concurrent_requests": &stat.AverageProxiedConcurrentRequests,
		"queue_requests_per_second":                 &stat.RequestCount,
		"queue_proxied_operations_per_second":       &stat.ProxiedRequestCount,
	} {
		pm := prometheusMetric(metricFamilies, m)
		if pm == nil {
			return Stat{}, fmt.Errorf("could not find value for %s in response", m)
		}
		*pv = *pm.Gauge.Value

		if stat.PodName == "" {
			stat.PodName = prometheusLabel(pm.Label, "destination_pod")
			if stat.PodName == "" {
				return Stat{}, errors.New("could not find pod name in metric labels")
			}
		}
	}
	return stat, nil
}

// prometheusMetric returns the point of the first Metric of the MetricFamily
// with the given key from the given map. If there is no such MetricFamily or it
// has no Metrics, then returns nil.
func prometheusMetric(metricFamilies map[string]*dto.MetricFamily, key string) *dto.Metric {
	if metric, ok := metricFamilies[key]; ok && len(metric.Metric) > 0 {
		return metric.Metric[0]
	}

	return nil
}

// prometheusLabels returns the value of the label with the given key from the
// given slice of labels. Returns an empty string if the label cannot be found.
func prometheusLabel(labels []*dto.LabelPair, key string) string {
	for _, label := range labels {
		if *label.Name == key {
			return *label.Value
		}
	}

	return ""
}
