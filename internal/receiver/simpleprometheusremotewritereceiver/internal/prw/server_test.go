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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/testdata"
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/transport"
)

func TestSmoke(t *testing.T) {
	mc := make(chan pmetric.Metrics)
	defer close(mc)
	timeout := 5 * time.Second
	addr := confignet.NetAddr{
		Endpoint:  "localhost:0",
		Transport: "tcp",
	}
	reporter := NewMockReporter(0)
	cfg := NewPrwConfig(
		addr,
		"metrics",
		timeout,
		reporter,
	)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	receiver, err := NewPrometheusRemoteWriteServer(ctx, cfg, mc)
	assert.Nil(t, err)
	require.NotNil(t, receiver)

	go func() { require.Nil(t, receiver.ListenAndServe()) }()

	closeAfter := 2 * time.Second
	t.Logf("will close after %d seconds, starting at %d", closeAfter/time.Second, time.Now().Unix())

	select {
	case <-mc:
		require.Fail(t, "Should not be sending metrics in this test")
	case <-time.After(closeAfter):
		t.Logf("Closed at %d!", time.Now().Unix())
		require.Nil(t, receiver.Shutdown(ctx))
	case <-time.After(timeout + 2*time.Second):
		require.Fail(t, "Should have closed server by now")
	case <-ctx.Done():
		assert.Error(t, ctx.Err())
	}
}

func TestWrite(t *testing.T) {
	mc := make(chan pmetric.Metrics)
	defer close(mc)
	testTimeout := 5 * time.Minute
	port, err := transport.GetFreePort()
	require.Nil(t, err)

	addr := confignet.NetAddr{
		Endpoint:  fmt.Sprintf("localhost:%d", port),
		Transport: "tcp",
	}
	reporter := NewMockReporter(0)
	cfg := NewPrwConfig(
		addr,
		"/metrics",
		5*time.Second,
		reporter,
	)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	receiver, err := NewPrometheusRemoteWriteServer(ctx, cfg, mc)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	go func() { require.NoError(t, receiver.ListenAndServe()) }()

	// Receiver takes care of lifecycle for us.  Here, need ot ensure we consume from channel just in case
	metricsSeen := 0
	go func() {
		for {
			select {
			case metric := <-mc: // note that this is handled for us at the receiver level.  Need to ensure sufficient buffering, lest it block forever
				require.NotNil(t, metric)
				assert.Greater(t, metric.DataPointCount(), 0)
				assert.Greater(t, metric.MetricCount(), 0)
				assert.NotNil(t, metric.ResourceMetrics())
				metricsSeen++
			case <-time.After(testTimeout + 2*time.Second):
				require.NotNil(t, receiver.Shutdown(ctx))
				require.Fail(t, "Should have closed server by now")
				return
			case <-ctx.Done():
				assert.Error(t, ctx.Err())
				return
			}
		}
	}()

	client, err := transport.NewMockPrwClient(addr.Endpoint, "/metrics")
	require.NotNil(t, client)
	require.Nil(t, err)

	time.Sleep(2 * time.Second)
	for _, wq := range testdata.GetWriteRequests() {
		t.Logf("sending data...")
		reporter.AddExpected(len(wq.Timeseries))
		err := client.SendWriteRequest(wq)
		assert.Nil(t, err, "Got error in server: %s", err)
	}
	go func() { require.Nil(t, receiver.Shutdown(ctx)) }()
	require.Greater(t, metricsSeen, 0)
}
