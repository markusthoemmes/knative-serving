/*
Copyright 2018 The Knative Authors

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
	"math"
	"sync"
	"time"

	"github.com/knative/serving/pkg/reconciler"

	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	// bucketSize is the size of the buckets of stats we create.
	bucketSize time.Duration = 2 * time.Second
)

// Autoscaler stores current state of an instance of an autoscaler
type Autoscaler struct {
	*DynamicConfig
	namespace       string
	revisionName    string
	endpointsLister corev1listers.EndpointsLister
	reporter        StatsReporter

	metricClient MetricClient

	// State in panic mode. Carries over multiple Scale calls.
	panicTime    *time.Time
	maxPanicPods int32

	// targetMutex guards the elements in the block below.
	targetMutex sync.RWMutex
	target      float64
}

// New creates a new instance of autoscaler
func New(
	dynamicConfig *DynamicConfig,
	namespace string,
	revisionName string,
	endpointsInformer corev1informers.EndpointsInformer,
	target float64,
	reporter StatsReporter,
	metricClient MetricClient) (*Autoscaler, error) {
	if endpointsInformer == nil {
		return nil, errors.New("'endpointsEnformer' must not be nil")
	}
	return &Autoscaler{
		DynamicConfig:   dynamicConfig,
		namespace:       namespace,
		revisionName:    revisionName,
		endpointsLister: endpointsInformer.Lister(),
		target:          target,
		reporter:        reporter,
		metricClient:    metricClient,
	}, nil
}

// Update reconfigures the UniScaler according to the MetricSpec.
func (a *Autoscaler) Update(spec MetricSpec) error {
	a.targetMutex.Lock()
	defer a.targetMutex.Unlock()
	a.target = spec.TargetConcurrency
	return nil
}

// Scale calculates the desired scale based on current statistics given the current time.
func (a *Autoscaler) Scale(ctx context.Context, now time.Time) (int32, bool) {
	logger := logging.FromContext(ctx)

	originalReadyPodsCount, err := readyPodsCountOfEndpoints(a.endpointsLister, a.namespace, reconciler.GetServingK8SServiceNameForObj(a.revisionName))
	if err != nil {
		logger.Errorw("Failed to get Endpoints via K8S Lister", zap.Error(err))
		return 0, false
	}
	// Use 1 if there are zero current pods.
	readyPodsCount := math.Max(1, float64(originalReadyPodsCount))

	config := a.Current()

	observedStableConcurrency, err := a.metricClient.GetStableConcurrency(a.namespace, a.revisionName)
	if err != nil {
		logger.Errorw("Failed to fetch stable concurrency", zap.Error(err))
	}
	observedPanicConcurrency, err := a.metricClient.GetPanicConcurrency(a.namespace, a.revisionName)
	if err != nil {
		logger.Errorw("Failed to fetch panic concurrency", zap.Error(err))
	}

	target := a.targetConcurrency()
	// Desired pod count is observed concurrency of the revision over desired (stable) concurrency per pod.
	// The scaling up rate is limited to the MaxScaleUpRate.
	desiredStablePodCount := a.podCountLimited(math.Ceil(observedStableConcurrency/target), readyPodsCount)
	desiredPanicPodCount := a.podCountLimited(math.Ceil(observedPanicConcurrency/target), readyPodsCount)

	a.reporter.ReportStableRequestConcurrency(observedStableConcurrency)
	a.reporter.ReportPanicRequestConcurrency(observedPanicConcurrency)
	a.reporter.ReportTargetRequestConcurrency(target)

	logger.Debugf("STABLE: Observed average %0.3f concurrency over %v seconds.", observedStableConcurrency, config.StableWindow)
	logger.Debugf("PANIC: Observed average %0.3f concurrency over %v seconds.", observedPanicConcurrency, config.PanicWindow)

	isOverPanicThreshold := observedPanicConcurrency/readyPodsCount >= target*2

	if a.panicTime == nil && isOverPanicThreshold {
		// Begin panicking when we cross the concurrency threshold in the panic window.
		logger.Info("PANICKING")
		a.panicTime = &now
	} else if a.panicTime != nil && !isOverPanicThreshold && a.panicTime.Add(config.StableWindow).Before(now) {
		// Stop panicking after the surge has made its way into the stable metric.
		logger.Info("Un-panicking.")
		a.panicTime = nil
		a.maxPanicPods = 0
	}

	var desiredPodCount int32
	if a.panicTime != nil {
		logger.Debug("Operating in panic mode.")
		a.reporter.ReportPanic(1)
		// We do not scale down while in panic mode. Only increases will be applied.
		if desiredPanicPodCount > a.maxPanicPods {
			logger.Infof("Increasing pods from %v to %v.", originalReadyPodsCount, desiredPanicPodCount)
			a.panicTime = &now
			a.maxPanicPods = desiredPanicPodCount
		}
		desiredPodCount = a.maxPanicPods
	} else {
		logger.Debug("Operating in stable mode.")
		a.reporter.ReportPanic(0)
		desiredPodCount = desiredStablePodCount
	}

	a.reporter.ReportDesiredPodCount(int64(desiredPodCount))
	return desiredPodCount, true
}

func (a *Autoscaler) targetConcurrency() float64 {
	a.targetMutex.RLock()
	defer a.targetMutex.RUnlock()
	return a.target
}

func (a *Autoscaler) podCountLimited(desiredPodCount, currentPodCount float64) int32 {
	return int32(math.Min(desiredPodCount, a.Current().MaxScaleUpRate*currentPodCount))
}

// readyPodsCountOfEndpoints returns the ready IP count in the K8S Endpoints object returned by
// the given K8S Informer with given namespace and name. This is same as ready Pod count.
func readyPodsCountOfEndpoints(lister corev1listers.EndpointsLister, ns, name string) (int, error) {
	readyPods := 0
	endpoints, err := lister.Endpoints(ns).Get(name)
	if apierrors.IsNotFound(err) {
		// Treat not found as zero endpoints, it either hasn't been created
		// or it has been torn down.
	} else if err != nil {
		return 0, err
	} else {
		for _, es := range endpoints.Subsets {
			readyPods += len(es.Addresses)
		}
	}

	return readyPods, nil
}
