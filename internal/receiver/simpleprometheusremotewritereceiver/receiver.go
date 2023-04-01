package simpleprometheusremotewritereceiver

import (
	"context"
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/transport"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"net"
	"time"
)

// TOOD hughesjj wait why is this a thing?
var _ receiver.Metrics = (*simplePrometheusWriteReceiver)(nil)

// simplePrometheusWriteReceiver implements the receiver.Metrics for PrometheusRemoteWrite protocol.
type simplePrometheusWriteReceiver struct {
	settings receiver.CreateSettings
	config   *Config

	server   transport.Server
	reporter transport.Reporter
	//parser       protocol.Parser
	nextConsumer consumer.Metrics
	cancel       context.CancelFunc
}

// New creates the PrometheusRemoteWrite receiver with the given parameters.
func New(
	set receiver.CreateSettings,
	config Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	rep, err := newReporter(set)
	if err != nil {
		return nil, err
	}

	r := &simplePrometheusWriteReceiver{
		settings:     set,
		config:       &config,
		nextConsumer: nextConsumer,
		reporter:     rep,
		parser:       &protocol.PrometheusWriteReceiver{},
	}
	return r, nil
}

// Start starts a UDP server that can process StatsD messages.
func (r *simplePrometheusWriteReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)
	server, err := r.server(*r.config)
	if err != nil {
		return err
	}
	r.server = server
	transferChan := make(chan transport.Metric, 10)
	ticker := time.NewTicker(r.config.AggregationInterval)
	err = r.parser.Initialize(
		r.config.EnableMetricType,
		r.config.IsMonotonicCounter,
		r.config.TimerHistogramMapping,
		// TODO check all here
	)
	if err != nil {
		return err
	}
	go func() {
		if err := r.server.ListenAndServe(r.parser, r.nextConsumer, r.reporter, transferChan); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				host.ReportFatalError(err)
			}
		}
	}()
	go func() {
		for {
			select {
			case <-ticker.C:
				// TODO hughesjj This happens periodically, not sure we actually want it
				batchMetrics := r.parser.GetMetrics()
				for _, batch := range batchMetrics {
					batchCtx := client.NewContext(ctx, batch.Info)
					r.Flush(batchCtx, batch.Metrics, r.nextConsumer)
				}
			case metric := <-transferChan:
				// TODO hughesjj
				_ = r.parser.Aggregate(metric.Raw, metric.Addr)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// Shutdown stops the PrometheusSimpleRemoteWrite receiver.
func (r *simplePrometheusWriteReceiver) Shutdown(context.Context) error {
	if r.cancel == nil {
		return nil
	}
	err := r.server.Close()
	r.cancel()
	return err
}

func (r *simplePrometheusWriteReceiver) Flush(ctx context.Context, metrics pmetric.Metrics, nextConsumer consumer.Metrics) error {
	error := nextConsumer.ConsumeMetrics(ctx, metrics)
	if error != nil {
		return error
	}

	return nil
}
