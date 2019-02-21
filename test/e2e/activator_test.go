// +build e2e

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

package e2e

import (
	"fmt"
	"testing"
	"time"
	"sync/atomic"
	"net/http"
	"golang.org/x/sync/errgroup"

	"github.com/knative/pkg/test/logging"
	"github.com/knative/pkg/test/spoof"
	"github.com/knative/serving/test"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	pkgTest "github.com/knative/pkg/test"
)

// TestActivatorOverload makes sure that activator can handle the load when scaling from 0.
// We need to add a similar test for the User pod overload once the second part of overload handling is done.
func TestActivatorOverload(t *testing.T) {
	var (
		logger  = logging.GetContextLogger(t.Name())
		clients = Setup(t)
	)
	const (
		// The number of concurrent requests to hit the activator with.
		// 1000 = the number concurrent connections in Istio.
		concurrency = 1000
		// Timeout to wait for the responses.
		// Ideally we must wait ~30 seconds, TODO: need to figure out where the delta comes from.
		timeout = 65 * time.Second
		// How long the service will process the request in ms.
		serviceSleep = 300
	)

	names := test.ResourceNames{
		Service: test.AppendRandomString(configName),
		Image:   "observed-concurrency",
	}

	configOptions := test.Options{
		ContainerConcurrency: 1,
	}

	fopt := func(service *v1alpha1.Service) {
		service.Spec.RunLatest.Configuration.RevisionTemplate.Annotations = map[string]string{"autoscaling.knative.dev/maxScale": "10"}
	}

	test.CleanupOnInterrupt(func() { test.TearDown(clients, names) })
	defer test.TearDown(clients, names)

	logger.Info("Creating a service with run latest configuration.")
	// Create a service with concurrency 1 that could sleep for N ms.
	// Limit its maxScale to 10 containers, wait for the service to scale down and hit it with concurrent requests.
	resources, err := test.CreateRunLatestServiceReady(t, clients, &names, &configOptions, fopt)
	if err != nil {
		t.Fatalf("Unable to create resources: %v", err)
	}
	domain := resources.Route.Status.Domain

	deploymentName := resources.Revision.Name + "-deployment"

	logger.Info("Waiting for deployment to scale to zero.")
	if err := pkgTest.WaitForDeploymentState(
		clients.KubeClient,
		deploymentName,
		test.DeploymentScaledToZeroFunc,
		"DeploymentScaledToZero",
		test.ServingNamespace,
		2*time.Minute); err != nil {
		t.Fatalf("Failed waiting for deployment to scale to zero: %v", err)
	}

	url := fmt.Sprintf("http://%s/?timeout=%d", domain, serviceSleep)

	client, err := pkgTest.NewSpoofingClient(clients.KubeClient, t.Logf, domain, test.ServingFlags.ResolvableDomain)
	client.RequestTimeout = timeout

	sendRequests(client, url, concurrency, timeout, logger, t)
}

func sendRequests(client *spoof.SpoofingClient, url string, concurrency int, timeout time.Duration, logger *logging.BaseLogger, t *testing.T) {
	t.Helper()
	var (
		group        errgroup.Group
		responses    int32
		wantResponse = http.StatusOK
		resChannel   = make(chan *spoof.Response, concurrency)
		errChan      = make(chan error)
		timeoutChan  = time.After(timeout)
	)

	// Send out the requests asynchronously and wait for them to finish.
	logger.Info("Starting to send out the requests")

	// Print out stats until one of these occurs:
	// - error happened in sending out requests,
	// - the global timeout is reached,
	// - we reached the number of required responses.
	go func() {
		for {
			select {
			case <-errChan:
				return
			case <-timeoutChan:
				return
			case <-time.Tick(5 * time.Second):
				logger.Infof("Received responses: %d", atomic.LoadInt32(&responses))
				if int32(concurrency) == atomic.LoadInt32(&responses) {
					return
				}
			}
		}
	}()

	// Send requests async and wait for the responses.
	for i := 0; i < concurrency; i++ {
		group.Go(func() error {
			req, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("error creating http request: %v", err)
			}
			res, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("unexpected error sending a request, %v\n", err)
			}
			atomic.AddInt32(&responses, 1)
			resChannel <- res
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		errChan <- fmt.Errorf("unexpected error making requests against activator: %v", err)
	}
	logger.Info("Done sending out requests")
	close(resChannel)

	logger.Info("Process the responses")

	for {
		select {
		case resp, ok := <-resChannel:
			if !ok {
				// The channel is closed, no more responses.
				return
			}
			if resp != nil {
				if resp.StatusCode != wantResponse {
					t.Errorf("Response code = %d, want: %d", resp.StatusCode, wantResponse)
				}
			} else {
				t.Errorf("No response code received for the request")
			}
		case err := <-errChan:
			t.Fatalf("Error happened while waiting for the responses: %v", err)
		}
	}
}
