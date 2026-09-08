package tool

import (
	"bytes"
	"net/http"
)

type HttpFactory struct {
	URL    string
	APIKey string
}

func (h *HttpFactory) Set(url string, apiKey string) {
	h.URL = url
	h.APIKey = apiKey
}

func (h *HttpFactory) Create(jsonData []byte) (*http.Request, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		h.URL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.APIKey)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}
