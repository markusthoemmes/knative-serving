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
package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
	pkgTest "knative.dev/pkg/reconciler/testing"
	"knative.dev/serving/pkg/activator/util"
	"knative.dev/serving/pkg/network"
)

func TestRequestEventHandler(t *testing.T) {
	namespace := "testspace"
	revision := "testrevision"
	fakeClock := &pkgTest.FakeClock{}

	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fakeClock.Time = fakeClock.Time.Add(1 * time.Second)
	})

	reqChan := make(chan network.ReqEvent, 2)
	handler := &RequestEventHandler{
		nextHandler: baseHandler,
		reqChan:     reqChan,
		clock:       fakeClock,
	}

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString(""))
	ctx := util.WithRevID(context.Background(), types.NamespacedName{Namespace: namespace, Name: revision})

	before := fakeClock.Time
	handler.ServeHTTP(resp, req.WithContext(ctx))

	in := <-handler.reqChan
	wantIn := network.ReqEvent{
		Key:  types.NamespacedName{Namespace: namespace, Name: revision},
		Type: network.ReqIn,
		Time: before,
	}

	if !cmp.Equal(wantIn, in) {
		t.Errorf("Unexpected in event (-want +got): %s", cmp.Diff(wantIn, in))
	}

	out := <-handler.reqChan
	wantOut := network.ReqEvent{
		Key:  wantIn.Key,
		Type: network.ReqOut,
		Time: before.Add(1 * time.Second),
	}

	if !cmp.Equal(wantOut, out) {
		t.Errorf("Unexpected out event (-want +got): %s", cmp.Diff(wantOut, out))
	}
}
