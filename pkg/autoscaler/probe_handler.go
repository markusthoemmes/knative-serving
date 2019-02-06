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
	"net/http"
)

const (
	// ActivatorProbeHeader is the header key for special requests to determine that the activator is present
	ActivatorProbeHeader string = "knative-activator-probe"
)

type probingHandler struct {
	nextHandler http.Handler
	response string
}

func ProbingHandler(h http.Handler, response string) http.Handler {
	return &probingHandler{
		nextHandler: h,
		response: response,
	}
}

func (h *probingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(ActivatorProbeHeader) != "" {
		w.Write([]byte(h.response))
		return
	}
	h.nextHandler.ServeHTTP(w, r)
}