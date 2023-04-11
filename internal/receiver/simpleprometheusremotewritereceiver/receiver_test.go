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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func TestHappy(t *testing.T) {
	timeout := time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cfg := createDefaultConfig().(*Config)
	cfg.ListenAddr.Endpoint = "localhost:0"
	var mockSettings receiver.CreateSettings
	var mockConsumer consumer.Metrics
	var host component.Host
	rec, err := createMetricsReceiver(ctx, mockSettings, cfg, mockConsumer)
	assert.NotNil(t, rec)
	assert.Nil(t, err)

	go func() {
		assert.Nil(t, rec.Start(ctx, host))
	}()

	select {
	case <-time.After(timeout + 2*time.Second):
		assert.Fail(t, "Should have closed server by now")
	case <-ctx.Done():
		assert.Error(t, ctx.Err())
	}

}
