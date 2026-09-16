package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
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

func (d *DeepSeek) BuildRequest(
	req requestprotocol.Request,
) ([]byte, error) {
	type deepSeekMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	requestData := struct {
		Model       string            `json:"model"`
		Messages    []deepSeekMessage `json:"messages"`
		Temperature *float64          `json:"temperature,omitempty"`
		TopP        *float64          `json:"top_p,omitempty"`
		MaxTokens   *int              `json:"max_tokens,omitempty"`
		Stream      bool              `json:"stream,omitempty"`
		Stop        []string          `json:"stop,omitempty"`
	}{
		Model:       req.Model,
		Messages:    make([]deepSeekMessage, len(req.Messages)),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Stop:        req.Stop,
	}

	for i, message := range req.Messages {
		requestData.Messages[i] = deepSeekMessage{
			Role:    message.Role,
			Content: message.Content,
		}
	}

	return json.Marshal(requestData)
}

func (d *DeepSeek) Chat(ctx context.Context, request requestprotocol.Request) ([]byte, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			ctx,
			time.Second*5,
		)
		defer cancel()
	}
	jsonData, err := d.BuildRequest(request)
	if err != nil {
		return nil, err
	}
	factory := &tool.HttpFactory{}
	factory.Set(
		"https://api.deepseek.com/chat/completions",
		d.apiKey,
	)

	req, err := factory.Create(jsonData)

	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

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
