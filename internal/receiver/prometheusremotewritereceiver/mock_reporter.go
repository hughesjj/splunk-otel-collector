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
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// mockReporter provides a reporter that provides some useful functionalities for
// tests (e.g.: wait for certain number of messages).
type mockReporter struct {
	TotalCalls         *int32
	OpsSuccess         *int32
	OpsStarted         *int32
	OpsFailed          *int32
	OpsSuccessComplete *int32
	OpsStartedComplete *int32
	OpsFailedComplete  *int32
	Errors             []error
}

var _ reporter = (*mockReporter)(nil)

func (m *mockReporter) AddExpectedError(newCalls int) int {
	atomic.AddInt32(m.OpsFailed, int32(newCalls))
	atomic.AddInt32(m.TotalCalls, int32(newCalls))
	return int(atomic.LoadInt32(m.TotalCalls))
}

func (m *mockReporter) AddExpectedSuccess(newCalls int) int {
	atomic.AddInt32(m.OpsSuccess, int32(newCalls))
	atomic.AddInt32(m.TotalCalls, int32(newCalls))
	return int(atomic.LoadInt32(m.TotalCalls))
}

func (m *mockReporter) AddExpectedStart(newCalls int) int {
	atomic.AddInt32(m.OpsStarted, int32(newCalls))
	atomic.AddInt32(m.TotalCalls, int32(newCalls))
	return int(atomic.LoadInt32(m.TotalCalls))
}

// newMockReporter returns a new instance of a mockReporter.
func newMockReporter(expectedOpStartedCalls int) *mockReporter {
	m := mockReporter{}
	atomic.AddInt32(m.OpsStarted, int32(expectedOpStartedCalls))
	atomic.AddInt32(m.TotalCalls, int32(expectedOpStartedCalls))
	return &m
}

func (m *mockReporter) StartMetricsOp(ctx context.Context) context.Context {
	atomic.AddInt32(m.OpsStartedComplete, 1)
	return ctx
}

func (m *mockReporter) OnError(_ context.Context, _ string, err error) {
	m.Errors = append(m.Errors, err)
	atomic.AddInt32(m.OpsFailedComplete, 1)
}

func (m *mockReporter) OnMetricsProcessed(_ context.Context, numReceivedMessages int, _ error) {
	atomic.AddInt32(m.OpsSuccessComplete, int32(numReceivedMessages))
}

func (m *mockReporter) OnDebugf(template string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(template, args...))
}

// WaitAllOnMetricsProcessedCalls blocks until the number of expected calls
// specified at creation of the otelReporter is completed.
func (m *mockReporter) WaitAllOnMetricsProcessedCalls(timeout time.Duration) error {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(timeout))
	defer cancel()

	go func() {
		totalExpected := atomic.LoadInt32(m.OpsSuccess) + atomic.LoadInt32(m.OpsFailed) + atomic.LoadInt32(m.OpsStarted)
		total := atomic.LoadInt32(m.OpsSuccessComplete) + atomic.LoadInt32(m.OpsFailedComplete) + atomic.LoadInt32(m.OpsStartedComplete)
		if total == totalExpected {
			cancel()
		} else {
			time.Sleep(time.Second)
		}
	}()

	for {
		select {
		case <-time.After(timeout):
			return errors.New("took too long to return")
		case <-ctx.Done():
			return nil
		}
	}
}
