package deepseek

import (
	"bytes"
	"io"
	"net/http"
)

func Chat(apiKey string, input string) (string, error) {
	jsonData := []byte(`{
		"model": "deepseek-v4-flash",
		"messages": [
			{
				"role": "user",
				"content": "你好"
			}
		]
	}`)
	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.deepseek.com/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(result), nil

}
