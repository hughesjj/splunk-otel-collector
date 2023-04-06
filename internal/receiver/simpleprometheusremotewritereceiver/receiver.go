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
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/prw"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"net"
	"sync"
)

// TODO hughesjj wait why is this a thing?
var _ receiver.Metrics = (*simplePrometheusWriteReceiver)(nil)

// simplePrometheusWriteReceiver implements the receiver.Metrics for PrometheusRemoteWrite protocol.
type simplePrometheusWriteReceiver struct {
	settings receiver.CreateSettings
	config   *Config

	server       prw.Server
	reporter     prw.Reporter
	nextConsumer consumer.Metrics
	cancel       context.CancelFunc

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
		config:       &config,
		nextConsumer: nextConsumer,
		reporter:     rep,
	}
	return r, nil
}
func (r *simplePrometheusWriteReceiver) buildTransportServer(ctx context.Context, metrics chan pmetric.Metrics) (prw.Server, error) {
	server, err := prw.NewPrometheusRemoteWriteReceiver(ctx, r.config, r.reporter, metrics)
	return server, err

}

// Start starts a UDP server that can process StatsD messages.
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
	err := r.server.Close()
	r.cancel()
	return err
}

func (r *simplePrometheusWriteReceiver) Flush(ctx context.Context, metrics pmetric.Metrics) error {
	err := r.nextConsumer.ConsumeMetrics(ctx, metrics)
	r.reporter.OnMetricsProcessed(ctx, metrics.DataPointCount(), err)
	return err
}
