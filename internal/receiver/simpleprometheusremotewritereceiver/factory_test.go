package simpleprometheusremotewritereceiver

import (
	"context"
	"fmt"
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/prw"
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/testdata"
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"testing"
	"time"
)

func TestFactory(t *testing.T) {
	timeout := time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cfg := createDefaultConfig().(*Config)
	//cfg := createDefaultConfig()
	freePort, err := getFreePort()
	require.Nil(t, err)

	cfg.ListenAddr.Endpoint = fmt.Sprintf("localhost:%d", freePort)
	cfg.ListenPath = "/metrics"

	sampleNoMdMetrics := testdata.GetWriteRequests()
	sampleMdMetrics := testdata.GetWriteRequestsWithMetadata()

	nopHost := componenttest.NewNopHost()
	mockSettings := receivertest.NewNopCreateSettings()
	mockConsumer := consumertest.NewNop()
	//receiver, err := createMetricsReceiver(ctx, mockSettings, cfg, mockConsumer)
	mockReporter := prw.NewMockReporter(len(sampleNoMdMetrics) + len(sampleMdMetrics))
	receiver, err := New(mockSettings, *cfg, mockConsumer)
	prwReceiver := receiver.(*simplePrometheusWriteReceiver)
	prwReceiver.reporter = mockReporter

	assert.Nil(t, err)
	require.NotNil(t, receiver)
	require.Nil(t, receiver.Start(ctx, nopHost))

	// Send some metrics
	client := transport.MockPrwClient{
		NetAddr: cfg.ListenAddr,
		Path:    "/metrics2", // TODO hughesjj does the path even matter given we're only server on this port?  Would mux let others share port?
	}

	// first try processing them without heuristics, then send them again with metadata.  check later to see if heuristics worked
	for _, wq := range sampleNoMdMetrics {
		require.Nil(t, client.SendWriteRequest(wq))
	}
	// TODO hughesjj now compare
	for _, wq := range sampleMdMetrics {
		require.Nil(t, client.SendWriteRequest(wq))
	}

	//closeAfter := math.Min(20 * time.Second, timeout - 5 * time.Second)
	closeAfter := 20 * time.Second
	t.Logf("will close after %d seconds, starting at %d", closeAfter/time.Second, time.Now().Unix())
	select {
	case <-time.After(closeAfter):
		t.Logf("Closing at %d!", time.Now().Unix())
		require.Nil(t, receiver.Shutdown(ctx))
	case <-time.After(timeout + 2*time.Second):
		require.Fail(t, "Should have closed server by now")
	case <-ctx.Done():
		assert.Error(t, ctx.Err())
	}

	// TODO hughesjj now compare
	// Prolly need to extend the reporter to inspect stuff
	mockReporter.WaitAllOnMetricsProcessedCalls(30 * time.Second)

}
