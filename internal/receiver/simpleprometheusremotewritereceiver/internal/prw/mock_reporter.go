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
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/transport"
	"sync"
	"time"
)

// MockReporter provides a Reporter that provides some useful functionalities for
// tests (eg.: wait for certain number of messages).
type MockReporter struct {
	//noCopy             noCopy
	wgMetricsProcessed sync.WaitGroup
	TotalCalls         int
	MessagesProcessed  int
}

func (m *MockReporter) AddExpected(newCalls int) int {
	m.wgMetricsProcessed.Add(newCalls)
	m.TotalCalls += newCalls
	return m.TotalCalls
}

var _ transport.Reporter = (*MockReporter)(nil)

// NewMockReporter returns a new instance of a MockReporter.
func NewMockReporter(expectedOnMetricsProcessedCalls int) *MockReporter {
	m := MockReporter{}
	m.wgMetricsProcessed.Add(expectedOnMetricsProcessedCalls)
	return &m
}

func (m *MockReporter) OnDataReceived(ctx context.Context) context.Context {
	return ctx
}

func (m *MockReporter) OnTranslationError(ctx context.Context, err error) {
}

func (m *MockReporter) OnMetricsProcessed(ctx context.Context, numReceivedMessages int, err error) {
	m.MessagesProcessed += numReceivedMessages
	m.wgMetricsProcessed.Done()
}

func (m *MockReporter) OnDebugf(template string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(template, args...))
}

// WaitAllOnMetricsProcessedCalls blocks until the number of expected calls
// specified at creation of the reporter is completed.
func (m *MockReporter) WaitAllOnMetricsProcessedCalls(timeout time.Duration) {
	m.wgMetricsProcessed.Wait()
}
