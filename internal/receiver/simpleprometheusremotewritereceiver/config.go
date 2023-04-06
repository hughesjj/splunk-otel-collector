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
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/tools"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
)

const (
	typeString = tools.TypeStr
)

type Config struct {
	// Needs address, path, timeout.  This is from SFX impl but we should prolly change it
	ListenAddr confignet.NetAddr `mapstructure:",squash"`
	ListenPath string            `mapstructure:",squash"`
	Timeout    *time.Duration    `mapstructure:",squash"`
}

func (c *Config) Validate() error {
	var errs error
	// TODO hughesjj impl validation
	return errs
}
