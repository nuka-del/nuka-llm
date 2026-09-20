package anthropic

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

type Cluade struct {
	apiKey  string
	version string
}

func (c *Cluade) ChangeVersion(v string) {
	c.version = v
}

func New(Apikey string) *Cluade {
	return &Cluade{
		apiKey: Apikey,
	}
}

func (c *Cluade) BuildRequest(
	req requestprotocol.Request,
) ([]byte, error) {
	if req.Model == "" {
		return nil, fmt.Errorf(
			"cluade:model is required",
		)
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf(
			"cluade: at least one message is required",
		)
	}
	var system string
	messages := make([]requestprotocol.Message, 0, len(req.Messages))

	for _, message := range req.Messages {
		if message.Role == "system" {
			if system != "" {
				system += "\n"
			}

			system += message.Content
			continue
		}

		messages = append(messages, message)
	}
	type CluadeMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	requestData := struct {
		Model       string          `json:"model"`
		System      string          `json:"system,omitempty"`
		Messages    []CluadeMessage `json:"messages"`
		Temperature *float64        `json:"temperature,omitempty"`
		TopP        *float64        `json:"top_p,omitempty"`
		MaxTokens   *int            `json:"max_tokens,omitempty"`
		Stream      bool            `json:"stream,omitempty"`
		Stop        []string        `json:"stop,omitempty"`
	}{
		Model:       req.Model,
		System:      system,
		Messages:    make([]CluadeMessage, len(messages)),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Stop:        req.Stop,
	}

	for i, message := range messages {
		requestData.Messages[i] = CluadeMessage{
			Role:    message.Role,
			Content: message.Content,
		}
	}
	if len(requestData.Messages) == 0 {
		return nil, fmt.Errorf(
			"cluade: at least one user or assistant message is required",
		)
	}

	return json.Marshal(requestData)
}
func (c *Cluade) Chat(ctx context.Context, request requestprotocol.Request) ([]byte, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			ctx,
			time.Second*5,
		)
		defer cancel()
	}
	jsonData, err := c.BuildRequest(request)
	if err != nil {
		return nil, err
	}
	factory := &tool.HttpFactory{}
	factory.Set(
		"https://api.anthropic.com/v1/messages",
		c.apiKey,
	)

	req, err := factory.Create(jsonData)
	if err != nil {
		return nil, err
	}
	req.Header.Del("Authorization")
	req.Header.Set("x-api-key", c.apiKey)
	if c.version == "" {
		c.ChangeVersion("2023-06-01")
	}
	req.Header.Set("anthropic-version", c.version)

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
			"cluade api error: status=%s, body=%s",
			resp.Status,
			string(result),
		)
	}

	return result, nil

}
