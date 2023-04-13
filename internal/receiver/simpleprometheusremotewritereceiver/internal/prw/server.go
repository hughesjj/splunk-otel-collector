// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prw

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/transport"
)

type PrometheusRemoteWriteReceiver struct {
	*http.Server
	handler
	sync.Mutex
}

type prwConfig struct {
	Reporter     transport.Reporter
	Addr         confignet.NetAddr
	Path         string
	Readtimeout  time.Duration
	Writetimeout time.Duration
}

func NewPrwConfig(address confignet.NetAddr, path string, timeout time.Duration, reporter transport.Reporter) *prwConfig {

	return &prwConfig{
		Addr:         address,
		Readtimeout:  timeout,
		Writetimeout: timeout,
		Reporter:     reporter,
		Path:         path,
	}
}

func NewPrometheusRemoteWriteReceiver(ctx context.Context, config prwConfig, mc chan pmetric.Metrics) (*PrometheusRemoteWriteReceiver, error) {
	parser, err := NewPrwOtelParser(config.Reporter)
	if nil != err {
		return nil, err
	}
	handler := newHandler(ctx, &parser, config.Reporter, config.Path, mc)
	server := http.Server{
		Handler:      handler,
		Addr:         config.Addr.Endpoint,
		ReadTimeout:  config.Readtimeout,
		WriteTimeout: config.Writetimeout,
	}
	return &PrometheusRemoteWriteReceiver{
		handler: *handler,
		Server:  &server,
	}, nil
}

func (prw *PrometheusRemoteWriteReceiver) Close() error {
	//prw.Lock()
	//defer prw.Unlock()
	return prw.Server.Close()
}

func (prw *PrometheusRemoteWriteReceiver) ListenAndServe() error {
	//prw.Lock()
	//defer prw.Unlock()
	prw.reporter.OnDebugf("Starting prometheus simple write server")
	return prw.Server.ListenAndServe()
}

type handler struct {
	parser   PrometheusRemoteOtelParser
	ctx      context.Context
	reporter transport.Reporter
	mc       chan pmetric.Metrics
	path     string
}

func newHandler(ctx context.Context, parser *PrometheusRemoteOtelParser, reporter transport.Reporter, path string, mc chan pmetric.Metrics) *handler {
	return &handler{
		ctx:      ctx,
		path:     path,
		parser:   *parser,
		reporter: reporter,
		mc:       mc,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != h.path {
		return
	}
	req, err := remote.DecodeWriteRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	results, err := h.parser.FromPrometheusWriteRequestMetrics(h.ctx, req)
	if nil != err {
		// Prolly server side errors too
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.reporter.OnTranslationError(h.ctx, err)
		return
	}
	h.mc <- results
	// In anticipation of eventually better supporting backpressure, return 202 instead of 204
	//w.WriteHeader(http.StatusAccepted)
	w.WriteHeader(http.StatusNoContent)
}
