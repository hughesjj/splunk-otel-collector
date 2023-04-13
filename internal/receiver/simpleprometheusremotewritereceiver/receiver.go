// Copyright Splunk, Inc.
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

package simpleprometheusremotewritereceiver

import (
	"context"
	"errors"
	"net"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/prw"
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/transport"
)

// TODO hughesjj wait why is this a thing?
var _ receiver.Metrics = (*simplePrometheusWriteReceiver)(nil)

// simplePrometheusWriteReceiver implements the receiver.Metrics for PrometheusRemoteWrite protocol.
type simplePrometheusWriteReceiver struct {
	server       transport.Server
	reporter     transport.Reporter
	nextConsumer consumer.Metrics
	cancel       context.CancelFunc
	settings     receiver.CreateSettings
	config       Config
	sync.Mutex
}

// New creates the PrometheusRemoteWrite receiver with the given parameters.
func New(
	settings receiver.CreateSettings,
	config Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	rep, err := newReporter(settings)
	if err != nil {
		return nil, err
	}

	r := &simplePrometheusWriteReceiver{
		settings:     settings,
		config:       config,
		nextConsumer: nextConsumer,
		reporter:     rep,
	}
	return r, nil
}
func (r *simplePrometheusWriteReceiver) buildTransportServer(ctx context.Context, metrics chan pmetric.Metrics) (transport.Server, error) {
	listener, err := net.Listen(r.config.ListenAddr.Transport, r.config.ListenAddr.Endpoint)
	if nil != err {
		return nil, err
	}
	defer listener.Close()
	cfg := prw.NewPrwConfig(
		r.config.ListenAddr,
		r.config.ListenPath,
		r.config.Timeout,
		r.reporter,
	)
	server, err := prw.NewPrometheusRemoteWriteReceiver(ctx, *cfg, metrics)
	return server, err

}

// Start starts an HTTP server that can process Prometheus Remote Write Requests
func (r *simplePrometheusWriteReceiver) Start(ctx context.Context, host component.Host) error {
	metricsChannel := make(chan pmetric.Metrics, 10)
	ctx, r.cancel = context.WithCancel(ctx)
	server, err := r.buildTransportServer(ctx, metricsChannel)
	if err != nil {
		return err
	}
	r.Lock()
	defer r.Unlock()
	if nil != r.server {
		err := r.server.Close()
		if err != nil {
			return err
		}
	}
	r.server = server
	// Start server
	go func(ctx context.Context) {
		if err := r.server.ListenAndServe(); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				host.ReportFatalError(err)
			}
		}
	}(ctx)
	// Manage server lifecycle
	go func(ctx context.Context) {
		for {
			select {
			case metrics := <-metricsChannel:
				err := r.Flush(ctx, metrics)
				if err != nil {
					r.reporter.OnTranslationError(ctx, err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	return nil
}

// Shutdown stops the PrometheusSimpleRemoteWrite receiver.
func (r *simplePrometheusWriteReceiver) Shutdown(context.Context) error {
	r.Lock()
	defer r.Unlock()
	if r.cancel == nil {
		return nil
	}
	defer r.cancel()
	return r.server.Close()
}

func (r *simplePrometheusWriteReceiver) Flush(ctx context.Context, metrics pmetric.Metrics) error {
	err := r.nextConsumer.ConsumeMetrics(ctx, metrics)
	r.reporter.OnMetricsProcessed(ctx, metrics.DataPointCount(), err)
	panic("cool at least we're getting stuff")
	return err
}
