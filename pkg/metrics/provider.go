package metrics

import (
	"time"

	apierr "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/metrics/pkg/apis/custom_metrics"

	"github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/provider"
	"github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/provider/helpers"

	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
)

// testingProvider is a sample implementation of provider.MetricsProvider which stores a map of fake metrics
type testingProvider struct {
	client dynamic.Interface
	mapper apimeta.RESTMapper
}

// NewFakeProvider returns an instance of testingProvider, along with its restful.WebService that opens endpoints to post new fake metrics
func NewFakeProvider(client dynamic.Interface, mapper apimeta.RESTMapper) provider.CustomMetricsProvider {
	provider := &testingProvider{
		client: client,
		mapper: mapper,
	}
	return provider
}

// valueFor is a helper function to get just the value of a specific metric
func (p *testingProvider) valueFor(info provider.CustomMetricInfo, name types.NamespacedName) (resource.Quantity, error) {
	info, _, err := info.Normalized(p.mapper)
	if err != nil {
		return resource.Quantity{}, err
	}

	return *resource.NewQuantity(30, resource.DecimalSI), nil
}

// metricFor is a helper function which formats a value, metric, and object info into a MetricValue which can be returned by the metrics API
func (p *testingProvider) metricFor(value resource.Quantity, name types.NamespacedName, info provider.CustomMetricInfo) (*custom_metrics.MetricValue, error) {
	objRef, err := helpers.ReferenceFor(p.mapper, name, info)
	if err != nil {
		return nil, err
	}

	return &custom_metrics.MetricValue{
		DescribedObject: objRef,
		MetricName:      info.Metric,
		Timestamp:       metav1.Time{time.Now()},
		Value:           value,
	}, nil
}

// metricsFor is a wrapper used by GetMetricBySelector to format several metrics which match a resource selector
func (p *testingProvider) metricsFor(namespace string, selector labels.Selector, info provider.CustomMetricInfo) (*custom_metrics.MetricValueList, error) {
	names, err := helpers.ListObjectNames(p.mapper, p.client, namespace, selector, info)
	if err != nil {
		return nil, err
	}

	res := make([]custom_metrics.MetricValue, 0, len(names))
	for _, name := range names {
		namespacedName := types.NamespacedName{Name: name, Namespace: namespace}
		value, err := p.valueFor(info, namespacedName)
		if err != nil {
			if apierr.IsNotFound(err) {
				continue
			}
			return nil, err
		}

		metric, err := p.metricFor(value, namespacedName, info)
		if err != nil {
			return nil, err
		}
		res = append(res, *metric)
	}

	return &custom_metrics.MetricValueList{
		Items: res,
	}, nil
}

func (p *testingProvider) GetMetricByName(name types.NamespacedName, info provider.CustomMetricInfo) (*custom_metrics.MetricValue, error) {

	value, err := p.valueFor(info, name)
	if err != nil {
		return nil, err
	}
	return p.metricFor(value, name, info)
}

func (p *testingProvider) GetMetricBySelector(namespace string, selector labels.Selector, info provider.CustomMetricInfo) (*custom_metrics.MetricValueList, error) {

	return p.metricsFor(namespace, selector, info)
}

func (p *testingProvider) ListAllMetrics() []provider.CustomMetricInfo {
	infos2 := []provider.CustomMetricInfo{
		{
			GroupResource: v1alpha1.Resource("Revision"),
			Namespaced:    true,
			Metric:        "concurrency",
		},
	}

	return infos2
}
