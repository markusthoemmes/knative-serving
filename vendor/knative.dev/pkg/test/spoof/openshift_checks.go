package spoof

import (
	"fmt"
	"net/http"
	"strings"
)

// isUnknownAuthority checks if the error contains "certificate signed by unknown authority".
// This error happens when OpenShift Route starts/changes to use passthrough mode. It takes a little bit time to be synced.
func isUnknownAuthority(err error) bool {
	return err != nil && strings.Contains(err.Error(), "certificate signed by unknown authority")
}

// RetryingRouteInconsistency retries common requests seen when creating a new route
// - 404 until the route is propagated to the proxy
// - 503 to account for Openshift route inconsistency (https://jira.coreos.com/browse/SRVKS-157)
func RouteInconsistencyRetryChecker() ResponseRetryChecker {
	const neededSuccesses = 5
	var successes int
	return func(resp *Response) (bool, error) {
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusServiceUnavailable {
			successes = 0
			return true, fmt.Errorf("retrying route inconsistency request: %s", resp)
		}
		successes++
		if successes < neededSuccesses {
			return true, fmt.Errorf("retrying to reach success confidence: successful %d, want at least %d", successes, neededSuccesses)
		}
		return false, nil
	}
}
