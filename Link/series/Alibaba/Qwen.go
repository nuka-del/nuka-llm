package alibaba

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	link "github.com/nuka-del/nuka-llm/Link"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
	tool "github.com/nuka-del/nuka-llm/Tool"
)

type Qwen struct {
	apiKey  string
	options link.Options
}

func New(Apikey string) *Qwen {
	return NewWithOptions(Apikey, link.DefaultOptions())
}

func NewWithOptions(apiKey string, options link.Options) *Qwen {
	return &Qwen{
		apiKey:  apiKey,
		options: options.WithDefaults(),
	}
}

func (d *Qwen) BuildRequest(
	req requestprotocol.Request,
) ([]byte, error) {
	if req.Model == "" {
		return nil, &sdkerror.SDKError{
			Provider: "qwen",
			Kind:     sdkerror.InvalidRequest,
			Message:  "model is required",
		}
	}
	if len(req.Messages) == 0 {
		return nil, &sdkerror.SDKError{
			Provider: "qwen",
			Kind:     sdkerror.InvalidRequest,
			Message:  "at least one message is required",
		}
	}
	type QwenMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type QwenProperty struct {
		Type        string `json:"type"`
		Description string `json:"description,omitempty"`
	}
	type QwenParameters struct {
		Type       string   `json:"type"`
		Properties any      `json:"properties,omitempty"`
		Required   []string `json:"required,omitempty"`
	}
	type QwenFunction struct {
		Name        string         `json:"name"`
		Description string         `json:"description,omitempty"`
		Parameters  QwenParameters `json:"parameters"`
	}
	type QwenTool struct {
		Type     string       `json:"type"`
		Function QwenFunction `json:"function"`
	}

	requestData := struct {
		Model       string        `json:"model"`
		Messages    []QwenMessage `json:"messages"`
		Temperature *float64      `json:"temperature,omitempty"`
		TopP        *float64      `json:"top_p,omitempty"`
		MaxTokens   *int          `json:"max_tokens,omitempty"`
		Stream      bool          `json:"stream,omitempty"`
		Stop        []string      `json:"stop,omitempty"`
		Tools       []QwenTool    `json:"tools,omitempty"`
	}{
		Model:       req.Model,
		Messages:    make([]QwenMessage, len(req.Messages)),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Stop:        req.Stop,
		Tools:       make([]QwenTool, len(req.Tools.ToolList)),
	}

	for i, message := range req.Messages {
		requestData.Messages[i] = QwenMessage{
			Role:    message.Role,
			Content: message.Content,
		}
	}

	for i, inputTool := range req.Tools.ToolList {
		inputParameters := inputTool.Function.Parameters

		var properties any
		if inputParameters.SimpleProperties != nil {
			simpleProperties := make(
				map[string]QwenProperty,
				len(inputParameters.SimpleProperties),
			)
			for name, property := range inputParameters.SimpleProperties {
				simpleProperties[name] = QwenProperty{
					Type:        property.PropertiesType,
					Description: property.Description,
				}
			}
			properties = simpleProperties
		} else if inputParameters.ConplexProproties != nil {
			properties = inputParameters.ConplexProproties
		}

		requestData.Tools[i] = QwenTool{
			Type: inputTool.ToolType,
			Function: QwenFunction{
				Name:        inputTool.Function.Name,
				Description: inputTool.Function.Description,
				Parameters: QwenParameters{
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
			Provider: "qwen",
			Kind:     sdkerror.InvalidRequest,
			Message:  "marshal request failed",
			Cause:    err,
		}
	}
	return jsonData, nil
}
func (q *Qwen) ChatRaw(ctx context.Context, request requestprotocol.Request) ([]byte, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && q.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			ctx,
			q.options.Timeout,
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
		return nil, &sdkerror.SDKError{
			Provider: "qwen",
			Kind:     sdkerror.InvalidRequest,
			Message:  "create HTTP request failed",
			Cause:    err,
		}
	}
	req = req.WithContext(ctx)

	resp, err := q.options.HTTPClient.Do(req)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: "qwen",
			Kind:     sdkerror.Transport,
			Message:  "send HTTP request failed",
			Cause:    err,
		}
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: "qwen",
			Kind:     sdkerror.Transport,
			Message:  "read response body failed",
			Cause:    err,
		}
	}

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &sdkerror.SDKError{
			Provider:   "qwen",
			Kind:       sdkerror.API,
			StatusCode: resp.StatusCode,
			Message:    "request returned unsuccessful status",
			Body:       string(result),
		}
	}

	return result, nil

}

func (q *Qwen) DecodeResponse(raw []byte) (responseprotocol.Response, error) {
	type chatCompletionResponse struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int    `json:"index"`
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: "qwen",
			Kind:     sdkerror.Decode,
			Message:  "decode response failed",
			Cause:    err,
		}
	}
	if len(decoded.Choices) == 0 {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: "qwen",
			Kind:     sdkerror.Decode,
			Message:  "response contains no choices",
		}
	}

	response := responseprotocol.Response{
		Provider: "qwen",
		ID:       decoded.ID,
		Model:    decoded.Model,
		Choices:  make([]responseprotocol.Choice, len(decoded.Choices)),
		Raw:      append(json.RawMessage(nil), raw...),
	}
	if decoded.Usage != nil {
		response.Usage = &responseprotocol.Usage{
			InputTokens:  decoded.Usage.PromptTokens,
			OutputTokens: decoded.Usage.CompletionTokens,
			TotalTokens:  decoded.Usage.TotalTokens,
		}
	}

	for i, choice := range decoded.Choices {
		message := responseprotocol.Message{
			Role:    choice.Message.Role,
			Content: choice.Message.Content,
		}
		for j, toolCall := range choice.Message.ToolCalls {
			arguments := []byte(toolCall.Function.Arguments)
			if len(arguments) > 0 && !json.Valid(arguments) {
				return responseprotocol.Response{}, &sdkerror.SDKError{
					Provider: "qwen",
					Kind:     sdkerror.Decode,
					Message:  "decode tool call arguments failed",
					Cause:    fmt.Errorf("choice %d tool call %d contains invalid JSON", i, j),
				}
			}
			message.ToolCalls = append(message.ToolCalls, responseprotocol.ToolCall{
				ID:   toolCall.ID,
				Type: toolCall.Type,
				Function: responseprotocol.FunctionCall{
					Name:      toolCall.Function.Name,
					Arguments: append(json.RawMessage(nil), arguments...),
				},
			})
		}
		response.Choices[i] = responseprotocol.Choice{
			Index:        choice.Index,
			Message:      message,
			FinishReason: choice.FinishReason,
		}
	}

	return response, nil
}
