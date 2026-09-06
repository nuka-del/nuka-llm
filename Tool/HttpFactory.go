package tool

import (
	"bytes"
	"net/http"
)

type HttpFactory struct {
	Url    string
	Apikey string
}

func (h *HttpFactory) Set(url string, apikey string) {
	h.Url = url
	h.Apikey = apikey
}

func (h *HttpFactory) Create(jsonData []byte) (*http.Request, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		h.Url,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.Apikey)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}
