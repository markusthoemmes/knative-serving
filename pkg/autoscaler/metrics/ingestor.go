/*
Copyright 2020 The Knative Authors

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

package metrics

import (
	"io"
	"net"

	grpc "google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/types"
)

// IngestorServer implements MetricIngestorServer interface.
type IngestorServer struct {
	StatsCh chan<- StatMessage
	server  *grpc.Server
}

// Ingest reads the stream of incoming WireStatMessage objects, translates them into
// StatMessage objects and surfaces those on the given channel.
func (s *IngestorServer) Ingest(srv MetricIngestor_IngestServer) error {
	for {
		req, err := srv.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		if req.Stat == nil {
			return nil
		}

		// Translate wire type into actual type.
		s.StatsCh <- StatMessage{
			Key: types.NamespacedName{
				Namespace: req.Namespace,
				Name:      req.Name,
			},
			Stat: *req.Stat,
		}
	}
}

// ListenAndServe listens on the address s.addr and handles incoming connections.
// It blocks until the server fails or Shutdown is called.
// It returns an error or, if Shutdown was called, nil.
func (s *IngestorServer) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.server = grpc.NewServer()
	RegisterMetricIngestorServer(s.server, s)
	return s.server.Serve(lis)
}

// Shutdown gracefully shuts the server down.
func (s *IngestorServer) Shutdown() {
	s.server.GracefulStop()
}
