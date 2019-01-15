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

package health

import (
	"io"
	"net/http"
	"sync"
)

// HealthState holds state about the current healthiness of the component.
type HealthState struct {
	alive        bool
	shuttingDown bool
	mutex        sync.RWMutex
}

// IsAlive returns whether or not the health server is in a known
// working state currently.
func (h *HealthState) IsAlive() bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return h.alive
}

// IsShuttingDown returns whether or not the health server is currently
// shutting down.
func (h *HealthState) IsShuttingDown() bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return h.shuttingDown
}

// setAlive updates the state to declare the service alive.
func (h *HealthState) setAlive() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.alive = true
	h.shuttingDown = false
}

// shutdown updates the state to declare the service shutting down.
func (h *HealthState) shutdown() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.alive = false
	h.shuttingDown = true
}

// HealthHandler constructs a handler that returns the current state of
// the health server.
func (h *HealthState) HealthHandler(prober func() bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sendAlive := func() {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "alive: true")
		}

		sendNotAlive := func() {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "alive: false")
		}

		if h.IsAlive() {
			sendAlive()
			return
		}

		if h.IsShuttingDown() {
			sendNotAlive()
			return
		}

		if prober != nil && !prober() {
			sendNotAlive()
			return
		}

		h.setAlive()
		sendAlive()
	}
}

// QuitHandler constructs a handler that shuts the current server down.
// Optional cleanup logic can be added via the given cleaner function.
func (h *HealthState) QuitHandler(cleaner func()) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		h.shutdown()

		if cleaner != nil {
			cleaner()
		}

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "alive: false")
	}
}
