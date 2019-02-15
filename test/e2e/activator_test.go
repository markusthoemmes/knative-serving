package e2e

import (
	"fmt"
	"testing"
	"time"
	"sync/atomic"
	"net/http"

	"github.com/knative/pkg/test/logging"
	"github.com/knative/pkg/test/spoof"
	"github.com/knative/serving/test"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	pkgTest "github.com/knative/pkg/test"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestActivatorOverload makes sure that activator can handle the load when scaling from 0.
// We need to add a similar test for the User pod overload once the second part of overload handling is done.
func TestActivatorOverload(t *testing.T) {
	var (
		clients *test.Clients
		logger  *logging.BaseLogger
	)
	// Create a service with concurrency 1 that could sleep for N ms.
	// Limit its maxScale to 10 containers, wait for the service to scale down and hit it with concurrent requests.
	logger = logging.GetContextLogger(t.Name())
	clients = Setup(t)

	helloWorldNames := test.ResourceNames{
		Config: test.AppendRandomString(configName, logger),
		Route:  test.AppendRandomString(routeName, logger),
		Image:  "observed-concurrency",
	}

	configOptions := test.Options{
		ContainerConcurrency: 1,
	}

	fopt := func(config *v1alpha1.Configuration) {
		config.Spec.RevisionTemplate.Annotations = make(map[string]string)
		config.Spec.RevisionTemplate.Annotations["autoscaling.knative.dev/maxScale"] = "10"
	}

	if _, err := test.CreateConfiguration(logger, clients, helloWorldNames, &configOptions, fopt); err != nil {
		t.Fatalf("Failed to create Configuration: %v", err)
	}

	if _, err := test.CreateRoute(logger, clients, helloWorldNames); err != nil {
		t.Fatalf("Failed to create Route: %v", err)
	}

	test.CleanupOnInterrupt(func() { TearDown(clients, helloWorldNames, logger) }, logger)
	defer TearDown(clients, helloWorldNames, logger)

	revision, err := test.WaitForConfigLatestRevision(clients, helloWorldNames)
	if err != nil {
		t.Fatalf("Configuration %s was not updated with the new revision: %v", helloWorldNames.Config, err)
	}

	logger.Info("When the Route reports as Ready, everything should be ready.")
	if err := test.WaitForRouteState(clients.ServingClient, helloWorldNames.Route, test.IsRouteReady, "RouteIsReady"); err != nil {
		t.Fatalf("The Route %s was not marked as Ready to serve traffic: %v", helloWorldNames.Route, err)
	}

	helloWorldRoute, err := clients.ServingClient.Routes.Get(helloWorldNames.Route, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get Route %q of helloworld app: %v", helloWorldNames.Route, err)
	}

	domain := helloWorldRoute.Status.Domain

	deploymentName := revision + "-deployment"

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

	// Hit the service with concurrent requests to provoke the start from 0,
	// the requests should go through activator, we make sure we receive only 200 responses.

	logger.Info("Waiting for endpoint to serve request")

	endpoint, err := spoof.GetServiceEndpoint(clients.KubeClient.Kube)
	if err != nil {
		t.Fatalf("Could not get service endpoint for spoofing client")
	}

	// Under ideal condition we should wait ~30 seconds, but we add some delta
	// to compensate the time k8s to provision the containers.
	// Given that the have sub-second pod start time, this values must be revisited.
	timeout := 65 * time.Second
	// Time the service will process the request in ms.
	serviceSleep := 300
	// 1000 requests = the number concurrent connections in Istio.
	concurrency := 1000

	url := fmt.Sprintf("http://%s/?timeout=%d", *endpoint, serviceSleep)

	client, err := pkgTest.NewSpoofingClient(clients.KubeClient, logger, domain, test.ServingFlags.ResolvableDomain)

	responseChannel := make(chan *spoof.Response, concurrency)
	roundTrip := roundTrip(client, url)

	sendRequests(roundTrip, concurrency, responseChannel, timeout, logger, t)

	analyseResponses(responseChannel, concurrency, timeout, t)
}

func sendRequests(roundTrip func() (*spoof.Response, error), concurrency int, resChannel chan *spoof.Response, timeout time.Duration, logger *logging.BaseLogger, t *testing.T) {
	var (
		group     errgroup.Group
		responses int32
	)
	doneChan := make(chan struct{})
	timeoutChan := time.After(timeout)
	errChan := make(chan error)

	// Send out the requests asynchronously and wait for them to finish.
	go func() {
		logger.Info("Starting to send out the requests")
		//	Send requests async and wait for the responses.
		for i := 0; i < concurrency; i++ {
			group.Go(func() error {
				res, err := roundTrip()
				if err != nil {
					return fmt.Errorf("Unexpected error sending a request, %v\n", err)
				}
				atomic.AddInt32(&responses, 1)
				resChannel <- res
				return nil
			})
		}
		err := group.Wait()
		if err != nil {
			errChan <- fmt.Errorf("Unexpected error making requests against activator: %v", err)
		}
		logger.Info("Done sending out requests")
		doneChan <- struct{}{}
	}()
	logger.Info("Waiting for all requests to finish")
	select {
	case <-doneChan:
		// success
	case err := <-errChan:
		t.Fatalf("Error happened while waiting for the responses: %v", err)
	case <-timeoutChan:
		t.Fatalf("Timed out after %s while collecting responses, collected responses %d, out of %d", timeout, atomic.LoadInt32(&responses), concurrency)
	}
	if atomic.LoadInt32(&responses) != int32(concurrency) {
		t.Fatalf("Responses from the activator, wanted %d, got %d", responses, concurrency)
	}
	logger.Info("All requests are finished")
}

func analyseResponses(respChan chan *spoof.Response, total int, timeout time.Duration, t *testing.T) {
	timeoutChan := time.After(timeout)
	wantResponse := 200
	for i := 0; i < total; i++ {
		select {
		case resp := <-respChan:
			if resp != nil {
				if resp.StatusCode != wantResponse {
					t.Errorf("Response code expected: %d, got %d", wantResponse, resp.StatusCode)
				}
			} else {
				t.Errorf("No response code received for the request")
			}
		case <-timeoutChan:
			t.Errorf("Timed out after %d while analyzing the responses", timeout)
		}
	}
}

func roundTrip(client *spoof.SpoofingClient, url string) func() (*spoof.Response, error) {
	return func() (*spoof.Response, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating http request: %v", err)
		}
		res, err := client.Do(req)
		return res, err
	}
}
