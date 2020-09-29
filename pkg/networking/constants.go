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

package networking

import "knative.dev/networking/pkg/apis/networking"

// The ports we setup on our services.
const (
	// ServicePortNameHTTP1Proxy is the name of the external port of the service for HTTP/1.1
	// when specifically targeting service pods even if the activator is in path.
	ServicePortNameHTTP1Proxy = networking.ServicePortNameHTTP1 + "-proxy"

	// ServicePortNameH2CProxy is the name of the external port of the service for HTTP/2
	// when specifically targeting service pods even if the activator is in path.
	ServicePortNameH2CProxy = networking.ServicePortNameH2C + "-proxy"

	// ServiceProxyPort is the port used when specifically targeting service pods even if
	// the activator is in path.
	ServiceProxyPort = 83

	// BackendHTTPPort is the backend, i.e. `targetPort` that we setup for HTTP services.
	BackendHTTPPort = 8012

	// BackendHTTP2Port is the backend, i.e. `targetPort` that we setup for HTTP services.
	BackendHTTP2Port = 8013

	// QueueAdminPort specifies the port number for
	// health check and lifecycle hooks for queue-proxy.
	QueueAdminPort = 8022

	// AutoscalingQueueMetricsPort specifies the port number for metrics emitted
	// by queue-proxy for autoscaler.
	AutoscalingQueueMetricsPort = 9090

	// UserQueueMetricsPort specifies the port number for metrics emitted
	// by queue-proxy for end user.
	UserQueueMetricsPort = 9091

	// ActivatorServiceName is the name of the activator Kubernetes service.
	ActivatorServiceName = "activator-service"

	// SKSLabelKey is the label key that SKS Controller attaches to the
	// underlying resources it controls.
	SKSLabelKey = networking.GroupName + "/serverlessservice"

	// ServiceTypeKey is the label key attached to a service specifying the type of service.
	// e.g. Public, Private.
	ServiceTypeKey = networking.GroupName + "/serviceType"
)

// ServicePortProxyName returns the proxy port for the app level protocol.
// This port is used to target the service pods directly even if the activator is in path.
func ServicePortProxyName(proto networking.ProtocolType) string {
	if proto == networking.ProtocolH2C {
		return ServicePortNameH2CProxy
	}
	return ServicePortNameHTTP1Proxy
}

// ServiceType is the enumeration type for the Kubernetes services
// that we have in our system, classified by usage purpose.
type ServiceType string

const (
	// ServiceTypePrivate is the label value for internal only services
	// for user applications.
	ServiceTypePrivate ServiceType = "Private"
	// ServiceTypePublic is the label value for externally reachable
	// services for user applications.
	ServiceTypePublic ServiceType = "Public"
)
