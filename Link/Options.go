package link

import (
	"net/http"
	"time"
)

// Options configures the shared HTTP behavior for a provider.
type Options struct {
	HTTPClient *http.Client
	Timeout    time.Duration
}

// DefaultOptions returns the current default HTTP client and request timeout.
func DefaultOptions() Options {
	return Options{
		HTTPClient: http.DefaultClient,
		Timeout:    5 * time.Second,
	}
}

// WithDefaults fills values that were not provided by the caller.
func (o Options) WithDefaults() Options {
	if o.HTTPClient == nil {
		o.HTTPClient = http.DefaultClient
	}
	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Second
	}
	return o
}
