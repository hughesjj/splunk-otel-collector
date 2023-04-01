package simpleprometheusremotewritereceiver

import (
	"go.opentelemetry.io/collector/config/confignet"
	"time"
)

const (
	typeString = "simpleprometheusremotewrite"
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
