package deepseek

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

	if req.Model == "" {
		return nil, &sdkerror.SDKError{
			Provider: "deepseek",
			Kind:     sdkerror.InvalidRequest,
			Message:  "model is required",
		}
	}
	if len(req.Messages) == 0 {
		return nil, &sdkerror.SDKError{
			Provider: "deepseek",
			Kind:     sdkerror.InvalidRequest,
			Message:  "at least one message is required",
		}
	}
	type deepSeekMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type deepSeekProperty struct {
		Type        string `json:"type"`
		Description string `json:"description,omitempty"`
	}
	type deepSeekParameters struct {
		Type       string   `json:"type"`
		Properties any      `json:"properties,omitempty"`
		Required   []string `json:"required,omitempty"`
	}
	type deepSeekFunction struct {
		Name        string             `json:"name"`
		Description string             `json:"description,omitempty"`
		Parameters  deepSeekParameters `json:"parameters"`
	}
	type deepSeekTool struct {
		Type     string           `json:"type"`
		Function deepSeekFunction `json:"function"`
	}

	requestData := struct {
		Model       string            `json:"model"`
		Messages    []deepSeekMessage `json:"messages"`
		Temperature *float64          `json:"temperature,omitempty"`
		TopP        *float64          `json:"top_p,omitempty"`
		MaxTokens   *int              `json:"max_tokens,omitempty"`
		Stream      bool              `json:"stream,omitempty"`
		Stop        []string          `json:"stop,omitempty"`
		Tools       []deepSeekTool    `json:"tools,omitempty"`
	}{
		Model:       req.Model,
		Messages:    make([]deepSeekMessage, len(req.Messages)),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Stop:        req.Stop,
		Tools:       make([]deepSeekTool, len(req.Tools.ToolList)),
	}

	for i, message := range req.Messages {
		requestData.Messages[i] = deepSeekMessage{
			Role:    message.Role,
			Content: message.Content,
		}
	}

	for i, inputTool := range req.Tools.ToolList {
		inputParameters := inputTool.Function.Parameters

		var properties any
		if inputParameters.SimpleProperties != nil {
			simpleProperties := make(
				map[string]deepSeekProperty,
				len(inputParameters.SimpleProperties),
			)
			for name, property := range inputParameters.SimpleProperties {
				simpleProperties[name] = deepSeekProperty{
					Type:        property.PropertiesType,
					Description: property.Description,
				}
			}
			properties = simpleProperties
		} else if inputParameters.ConplexProproties != nil {
			properties = inputParameters.ConplexProproties
		}

		requestData.Tools[i] = deepSeekTool{
			Type: inputTool.ToolType,
			Function: deepSeekFunction{
				Name:        inputTool.Function.Name,
				Description: inputTool.Function.Description,
				Parameters: deepSeekParameters{
					Type:       inputParameters.ParaType,
					Properties: properties,
					Required:   inputParameters.Required,
				},
			},
		}
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: "deepseek",
			Kind:     sdkerror.InvalidRequest,
			Message:  "marshal request failed",
			Cause:    err,
		}
	}
	return jsonData, nil
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
		return nil, &sdkerror.SDKError{
			Provider: "deepseek",
			Kind:     sdkerror.InvalidRequest,
			Message:  "create HTTP request failed",
			Cause:    err,
		}
	}
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: "deepseek",
			Kind:     sdkerror.Transport,
			Message:  "send HTTP request failed",
			Cause:    err,
		}
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: "deepseek",
			Kind:     sdkerror.Transport,
			Message:  "read response body failed",
			Cause:    err,
		}
	}

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &sdkerror.SDKError{
			Provider:   "deepseek",
			Kind:       sdkerror.API,
			StatusCode: resp.StatusCode,
			Message:    "request returned unsuccessful status",
			Body:       string(result),
		}
	}

	return result, nil

}
