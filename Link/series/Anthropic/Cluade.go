package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
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
		return nil, &sdkerror.SDKError{
			Provider: "anthropic",
			Kind:     sdkerror.InvalidRequest,
			Message:  "model is required",
		}
	}
	if len(req.Messages) == 0 {
		return nil, &sdkerror.SDKError{
			Provider: "anthropic",
			Kind:     sdkerror.InvalidRequest,
			Message:  "at least one message is required",
		}
	}
	if req.MaxTokens == nil {
		return nil, &sdkerror.SDKError{
			Provider: "anthropic",
			Kind:     sdkerror.InvalidRequest,
			Message:  "max_tokens is required",
		}
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
	type CluadeProperty struct {
		Type        string `json:"type"`
		Description string `json:"description,omitempty"`
	}
	type CluadeInputSchema struct {
		Type       string   `json:"type"`
		Properties any      `json:"properties,omitempty"`
		Required   []string `json:"required,omitempty"`
	}
	type CluadeTool struct {
		Name        string            `json:"name"`
		Description string            `json:"description,omitempty"`
		InputSchema CluadeInputSchema `json:"input_schema"`
	}

	requestData := struct {
		Model       string          `json:"model"`
		System      string          `json:"system,omitempty"`
		Messages    []CluadeMessage `json:"messages"`
		Temperature *float64        `json:"temperature,omitempty"`
		TopP        *float64        `json:"top_p,omitempty"`
		MaxTokens   *int            `json:"max_tokens"`
		Stream      bool            `json:"stream,omitempty"`
		Stop        []string        `json:"stop_sequences,omitempty"`
		Tools       []CluadeTool    `json:"tools,omitempty"`
	}{
		Model:       req.Model,
		System:      system,
		Messages:    make([]CluadeMessage, len(messages)),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Stop:        req.Stop,
		Tools:       make([]CluadeTool, len(req.Tools.ToolList)),
	}

	for i, message := range messages {
		requestData.Messages[i] = CluadeMessage{
			Role:    message.Role,
			Content: message.Content,
		}
	}

	for i, inputTool := range req.Tools.ToolList {
		inputParameters := inputTool.Function.Parameters

		var properties any
		if inputParameters.SimpleProperties != nil {
			simpleProperties := make(
				map[string]CluadeProperty,
				len(inputParameters.SimpleProperties),
			)
			for name, property := range inputParameters.SimpleProperties {
				simpleProperties[name] = CluadeProperty{
					Type:        property.PropertiesType,
					Description: property.Description,
				}
			}
			properties = simpleProperties
		} else if inputParameters.ConplexProproties != nil {
			properties = inputParameters.ConplexProproties
		}

		requestData.Tools[i] = CluadeTool{
			Name:        inputTool.Function.Name,
			Description: inputTool.Function.Description,
			InputSchema: CluadeInputSchema{
				Type:       inputParameters.ParaType,
				Properties: properties,
				Required:   inputParameters.Required,
			},
		}
	}

	if len(requestData.Messages) == 0 {
		return nil, &sdkerror.SDKError{
			Provider: "anthropic",
			Kind:     sdkerror.InvalidRequest,
			Message:  "at least one user or assistant message is required",
		}
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: "anthropic",
			Kind:     sdkerror.InvalidRequest,
			Message:  "marshal request failed",
			Cause:    err,
		}
	}
	return jsonData, nil
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
		return nil, &sdkerror.SDKError{
			Provider: "anthropic",
			Kind:     sdkerror.InvalidRequest,
			Message:  "create HTTP request failed",
			Cause:    err,
		}
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
		return nil, &sdkerror.SDKError{
			Provider: "anthropic",
			Kind:     sdkerror.Transport,
			Message:  "send HTTP request failed",
			Cause:    err,
		}
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: "anthropic",
			Kind:     sdkerror.Transport,
			Message:  "read response body failed",
			Cause:    err,
		}
	}

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &sdkerror.SDKError{
			Provider:   "anthropic",
			Kind:       sdkerror.API,
			StatusCode: resp.StatusCode,
			Message:    "request returned unsuccessful status",
			Body:       string(result),
		}
	}

	return result, nil

}
