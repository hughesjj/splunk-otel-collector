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
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/config"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"net"
	"net/http"
	"sync"

	"github.com/prometheus/prometheus/storage/remote"
)

type PrometheusRemoteWriteReceiver struct {
	handler
	net.Listener
	http.Server

	sync.Mutex
}

func NewPrometheusRemoteWriteReceiver(ctx context.Context, cfg *config.Config, reporter Reporter, mc chan pmetric.Metrics) (*PrometheusRemoteWriteReceiver, error) {
	parser, err := NewPrwOtelParser(reporter)
	if nil != err {
		return nil, err
	}
	listener, err := net.Listen(cfg.ListenAddr.Transport, cfg.ListenAddr.Endpoint)
	defer listener.Close()
	handler := newHandler(ctx, parser, reporter, mc)
	server := http.Server{
		Handler:      handler,
		Addr:         listener.Addr().String(),
		ReadTimeout:  *cfg.Timeout,
		WriteTimeout: *cfg.Timeout,
	}
	return &PrometheusRemoteWriteReceiver{
		handler:  *handler,
		Listener: listener,
		Server:   server,
	}, nil
}

func (prw *PrometheusRemoteWriteReceiver) Close() error {
	prw.Lock()
	defer prw.Unlock()
	serverErr := prw.Server.Close()
	protocolErr := prw.Listener.Close()
	if serverErr != nil && protocolErr != nil {
		return multierr.Combine(serverErr, protocolErr)
	}
	if serverErr != nil {
		return serverErr
	}

	return nil
}

func (prw *PrometheusRemoteWriteReceiver) ListenAndServe() error {
	prw.Lock()
	defer prw.Unlock()
	err := prw.Server.ListenAndServe()
	return err
}

type handler struct {
	ctx      context.Context
	parser   PrwOtelParser
	reporter Reporter
	mc       chan pmetric.Metrics
}

func newHandler(ctx context.Context, parser PrwOtelParser, reporter Reporter, mc chan pmetric.Metrics) *handler {
	return &handler{
		ctx:      ctx,
		parser:   parser,
		reporter: reporter,
		mc:       mc,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := remote.DecodeWriteRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	results, err := h.parser.FromPrometheusWriteRequestMetrics(h.ctx, req, h.reporter)
	if nil != err {
		// Prolly server side errors too
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.reporter.OnTranslationError(h.ctx, err)
		return
	}
	h.mc <- results
	// In anticipation of eventually better supporting backpressure, return 202 instead of 204
	w.WriteHeader(http.StatusAccepted)
}
