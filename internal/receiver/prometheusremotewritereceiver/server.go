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

package prometheusremotewritereceiver

import (
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type prometheusRemoteWriteServer struct {
	*http.Server
	*ServerConfig
	closeChannel *sync.Once
}

type ServerConfig struct {
	Reporter reporter
	component.Host
	Mc chan<- pmetric.Metrics
	component.TelemetrySettings
	Path   string
	Parser *PrometheusRemoteOtelParser
	confighttp.HTTPServerSettings
}

func newPrometheusRemoteWriteServer(ctx context.Context, config *ServerConfig) (*prometheusRemoteWriteServer, error) {
	mx := mux.NewRouter()
	handler := newHandler(ctx, config.Parser, config, config.Mc)
	mx.HandleFunc(config.Path, handler)
	mx.Host(config.Endpoint)
	server, err := config.HTTPServerSettings.ToServer(config.Host, config.TelemetrySettings, handler)
	// Currently this is not set, in favor of the pattern where they always explicitly pass the listener
	server.Addr = config.Endpoint
	if err != nil {
		return nil, err
	}
	return &prometheusRemoteWriteServer{
		Server:       server,
		ServerConfig: config,
		closeChannel: &sync.Once{},
	}, nil
}

func (prw *prometheusRemoteWriteServer) Close() error {
	defer prw.closeChannel.Do(func() { close(prw.Mc) })
	return prw.Server.Close()
}

func (prw *prometheusRemoteWriteServer) ListenAndServe() error {
	prw.Reporter.OnDebugf("Starting prometheus simple write server")
	listener, err := prw.ServerConfig.ToListener()
	if err != nil {
		return err
	}
	defer listener.Close()
	err = prw.Server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func newHandler(ctx context.Context, parser *PrometheusRemoteOtelParser, sc *ServerConfig, mc chan<- pmetric.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx2 := sc.Reporter.StartMetricsOp(ctx)
		req, err := remote.DecodeWriteRequest(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(req.Timeseries) == 0 && len(req.Metadata) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		results, err := parser.FromPrometheusWriteRequestMetrics(ctx2, req)
		if nil != err {
			// Prolly server side errors too
			http.Error(w, err.Error(), http.StatusBadRequest)
			sc.Reporter.OnError(ctx2, "prometheus_translation", err)
			return
		}
		// TODO hughesjj dude you suck at channels
		mc <- results // TODO hughesjj well, I think it might break here for some reason?
		// In anticipation of eventually better supporting backpressure, return 202 instead of 204
		// eh actually the prometheus remote write client doesn't support non 204...
		w.WriteHeader(http.StatusAccepted)
		sc.Reporter.OnMetricsProcessed(ctx2, results.MetricCount(), nil)
	}
}
