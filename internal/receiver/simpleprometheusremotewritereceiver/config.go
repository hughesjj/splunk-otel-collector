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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/tools"
)

var _ component.Config = (*Config)(nil)

const (
	typeString = tools.TypeStr
)

type Config struct {
	// TODO the other guy has some really cool stuff in here
	ListenAddr confignet.NetAddr `mapstructure:",squash"`
	ListenPath string            `mapstructure:",squash"`
	Timeout    time.Duration     `mapstructure:",squash"`
	BufferSize int               // Channel buffer size, defaults to blocking each request until processed
}
