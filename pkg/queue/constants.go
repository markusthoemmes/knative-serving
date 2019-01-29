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

package queue

const (
	// RequestQueueQuitPath specifies the path to send quit request to
	// queue-proxy. This is used for preStop hook of queue-proxy. It:
	// - marks the service as not ready, so that requests will no longer
	//   be routed to it,
	// - adds a small delay, so that the container doesn't get killed at
	//   the same time the pod is marked for removal.
	RequestQueueQuitPath = "/quitquitquit"

	// RequestQueueHealthPath specifies the path for health checks for
	// queue-proxy.
	RequestQueueHealthPath = "/health"

	// QueueProbeHeader is the header key for special requests to determine that the queue proxy is present
	QueueProbeHeader string = "knative-queue-probe"
)
