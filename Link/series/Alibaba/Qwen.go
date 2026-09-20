package alibaba

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

type Qwen struct {
	apiKey string
}

func New(Apikey string) *Qwen {
	return &Qwen{
		apiKey: Apikey,
	}
}

func (d *Qwen) BuildRequest(
	req requestprotocol.Request,
) ([]byte, error) {
	if req.Model == "" {
		return nil, fmt.Errorf(
			"qwen:model is required",
		)
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf(
			"qwen: at least one message is required",
		)
	}
	type QwenMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	requestData := struct {
		Model       string        `json:"model"`
		Messages    []QwenMessage `json:"messages"`
		Temperature *float64      `json:"temperature,omitempty"`
		TopP        *float64      `json:"top_p,omitempty"`
		MaxTokens   *int          `json:"max_tokens,omitempty"`
		Stream      bool          `json:"stream,omitempty"`
		Stop        []string      `json:"stop,omitempty"`
	}{
		Model:       req.Model,
		Messages:    make([]QwenMessage, len(req.Messages)),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Stop:        req.Stop,
	}

	for i, message := range req.Messages {
		requestData.Messages[i] = QwenMessage{
			Role:    message.Role,
			Content: message.Content,
		}
	}

	return json.Marshal(requestData)
}
func (q *Qwen) Chat(ctx context.Context, request requestprotocol.Request) ([]byte, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			ctx,
			time.Second*5,
		)
		defer cancel()
	}
	jsonData, err := q.BuildRequest(request)
	if err != nil {
		return nil, err
	}
	factory := &tool.HttpFactory{}
	factory.Set(
		"https://dashscope-intl.aliyuncs.com/compatible-mode/v1/chat/completions",
		q.apiKey,
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
			"qwen api error: status=%s, body=%s",
			resp.Status,
			string(result),
		)
	}

	return result, nil

}
