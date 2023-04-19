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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/prw"
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/testdata"
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/transport"
)

func TestHappy(t *testing.T) {
	timeout := time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cfg := createDefaultConfig().(*Config)
	freePort, err := transport.GetFreePort()
	require.Nil(t, err)

	cfg.ListenAddr.Endpoint = fmt.Sprintf("localhost:%d", freePort)
	cfg.ListenPath = "/metrics"

	sampleNoMdMetrics := testdata.GetWriteRequests()
	sampleMdMetrics := testdata.GetWriteRequestsWithMetadata()

	nopHost := componenttest.NewNopHost()
	mockSettings := receivertest.NewNopCreateSettings()
	mockConsumer := consumertest.NewNop()
	mockReporter := prw.NewMockReporter(len(sampleNoMdMetrics) + len(sampleMdMetrics))
	receiver, err := New(mockSettings, cfg, mockConsumer)
	prwReceiver := receiver.(*simplePrometheusWriteReceiver)
	prwReceiver.reporter = mockReporter

	assert.Nil(t, err)
	require.NotNil(t, prwReceiver)
	require.Nil(t, prwReceiver.Start(ctx, nopHost))

	// Send some metrics
	client, err := transport.NewMockPrwClient(
		cfg.ListenAddr.Endpoint,
		"metrics",
	)
	require.Nil(t, err)
	require.NotNil(t, client)

	// first try processing them without heuristics, then send them again with metadata.  check later to see if heuristics worked
	for index, wq := range sampleNoMdMetrics {
		mockReporter.AddExpected(1)
		err = client.SendWriteRequest(wq)
		assert.Nil(t, err, "failed to write %d", index)
		if nil != err {
			assert.Nil(t, errors.Unwrap(err))
		}
	}
	// TODO hughesjj now compare
	for index, wq := range sampleMdMetrics {
		mockReporter.AddExpected(1)
		err = client.SendWriteRequest(wq)
		assert.Nil(t, err, "failed to write %d reason %s", index, err)
	}

	require.Nil(t, prwReceiver.Shutdown(ctx))

	require.Nil(t, mockReporter.WaitAllOnMetricsProcessedCalls(30*time.Second))

}
