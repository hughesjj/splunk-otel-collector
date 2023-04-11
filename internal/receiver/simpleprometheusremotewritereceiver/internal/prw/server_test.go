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
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"go.opentelemetry.io/collector/config/confignet"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestSmoke(t *testing.T) {
	mc := make(chan pmetric.Metrics)
	timeout := 5 * time.Second
	addr := confignet.NetAddr{
		Endpoint:  "localhost:0",
		Transport: "tcp",
	}
	reporter := NewMockReporter(0)
	cfg := NewPrwConfig(
		addr,
		"/metrics",
		timeout,
		reporter,
	)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	receiver, err := NewPrometheusRemoteWriteReceiver(ctx, *cfg, mc)
	assert.Nil(t, err)
	require.NotNil(t, receiver)

	go func() {
		assert.Nil(t, receiver.ListenAndServe())
	}()

	closeAfter := 20 * time.Second
	t.Logf("will close after %d seconds, starting at %d", closeAfter/time.Second, time.Now().Unix())

	select {
	case <-time.After(closeAfter):
		t.Logf("Closed at %d!", time.Now().Unix())
		require.Nil(t, receiver.Shutdown(ctx))
	case <-time.After(timeout + 2*time.Second):
		require.Fail(t, "Should have closed server by now")
	case <-ctx.Done():
		assert.Error(t, ctx.Err())
	}

}
