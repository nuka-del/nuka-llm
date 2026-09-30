package link

import (
	"net/http"
	"time"
)

type Options struct {
	HTTPClient *http.Client
	Timeout    time.Duration
}

func DefaultOptions() Options {
	return Options{
		HTTPClient: http.DefaultClient,
		Timeout:    5 * time.Second,
	}
}

func (o Options) WithDefaults() Options {
	if o.HTTPClient == nil {
		o.HTTPClient = http.DefaultClient
	}
	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Second
	}
	return o
}
