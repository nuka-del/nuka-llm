package deepseek

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	tool "github.com/nuka-del/nuka-llm/Tool"
)

type DeepSeek struct {
	apiKey string
}

func New(apiKey string) *DeepSeek {
	return &DeepSeek{
		apiKey: apiKey,
	}
}

func (d *DeepSeek) Chat(input string, model string) ([]byte, error) {
	content, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	modelJSON, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}

	jsonData := []byte(`{
		"model": ` + string(modelJSON) + `,
		"messages": [
			{
				"role": "user",
				"content": ` + string(content) + `
			}
		]
	}`)
	factory := &tool.HttpFactory{}
	factory.Set(
		"https://api.deepseek.com/chat/completions",
		d.apiKey,
	)

	req, err := factory.Create(jsonData)

	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf(
			"deepseek api error: status=%s, body=%s",
			resp.Status,
			string(result),
		)
	}

	return result, nil

}
