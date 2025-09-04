package config

import (
	"encoding/json"

	"github.com/kelseyhightower/envconfig"
)

// Config represents service configuration for dis-bundle-scheduler
type Configuration struct {
	ServiceToken  string `envconfig:"BUNDLES_API_SERVICE_TOKEN"  json:"-"`
	BundlesAPIUrl string `envconfig:"BUNDLES_API_URL"`
}

var cfg *Configuration

// Get returns the config with variables loaded from environment variables
func Get() (*Configuration, error) {
	if cfg != nil {
		return cfg, nil
	}

	cfg = &Configuration{
		ServiceToken:  "bundle-scheduler-test-auth-token",
		BundlesAPIUrl: "http://localhost:29800",
	}

	return cfg, envconfig.Process("", cfg)
}

// String is implemented to prevent sensitive fields being logged.
// The config is returned as JSON with sensitive fields omitted.
func (config Configuration) String() string {
	b, _ := json.Marshal(config)
	return string(b)
}
