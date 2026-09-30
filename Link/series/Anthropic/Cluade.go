package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	link "github.com/nuka-del/nuka-llm/Link"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
	tool "github.com/nuka-del/nuka-llm/Tool"
)

type Cluade struct {
	apiKey  string
	version string
	options link.Options
}

func (c *Cluade) ChangeVersion(v string) {
	c.version = v
}

func New(Apikey string) *Cluade {
	return NewWithOptions(Apikey, link.DefaultOptions())
}

func NewWithOptions(apiKey string, options link.Options) *Cluade {
	return &Cluade{
		apiKey:  apiKey,
		options: options.WithDefaults(),
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
			systemContent, err := requestprotocol.ContentText(message.Content)
			if err != nil {
				return nil, &sdkerror.SDKError{
					Provider: "anthropic",
					Kind:     sdkerror.InvalidRequest,
					Message:  "encode system content failed",
					Cause:    err,
				}
			}
			if system != "" {
				system += "\n"
			}

			system += systemContent
			continue
		}

		messages = append(messages, message)
	}
	type CluadeMessage struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	type CluadeContentBlock struct {
		Type      string          `json:"type"`
		Text      string          `json:"text,omitempty"`
		ID        string          `json:"id,omitempty"`
		Name      string          `json:"name,omitempty"`
		Input     json.RawMessage `json:"input,omitempty"`
		ToolUseID string          `json:"tool_use_id,omitempty"`
		Content   string          `json:"content,omitempty"`
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
		Messages:    make([]CluadeMessage, 0, len(messages)),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Stop:        req.Stop,
		Tools:       make([]CluadeTool, len(req.Tools.ToolList)),
	}

	for i := 0; i < len(messages); {
		message := messages[i]

		if message.Role == "tool" {
			contentBlocks := make([]CluadeContentBlock, 0)
			for i < len(messages) && messages[i].Role == "tool" {
				toolResult := messages[i]
				if toolResult.ToolCallID == "" {
					return nil, &sdkerror.SDKError{
						Provider: "anthropic",
						Kind:     sdkerror.InvalidRequest,
						Message:  "tool result is missing tool_call_id",
					}
				}
				content, err := requestprotocol.ContentText(toolResult.Content)
				if err != nil {
					return nil, &sdkerror.SDKError{
						Provider: "anthropic",
						Kind:     sdkerror.InvalidRequest,
						Message:  "encode tool result failed",
						Cause:    err,
					}
				}
				contentBlocks = append(contentBlocks, CluadeContentBlock{
					Type:      "tool_result",
					ToolUseID: toolResult.ToolCallID,
					Content:   content,
				})
				i++
			}
			requestData.Messages = append(requestData.Messages, CluadeMessage{
				Role:    "user",
				Content: contentBlocks,
			})
			continue
		}

		content, err := requestprotocol.ContentText(message.Content)
		if err != nil {
			return nil, &sdkerror.SDKError{
				Provider: "anthropic",
				Kind:     sdkerror.InvalidRequest,
				Message:  "encode message content failed",
				Cause:    err,
			}
		}

		if len(message.ToolCalls) == 0 {
			requestData.Messages = append(requestData.Messages, CluadeMessage{
				Role:    message.Role,
				Content: content,
			})
			i++
			continue
		}

		if message.Role != "assistant" {
			return nil, &sdkerror.SDKError{
				Provider: "anthropic",
				Kind:     sdkerror.InvalidRequest,
				Message:  "tool calls must belong to an assistant message",
			}
		}

		contentBlocks := make([]CluadeContentBlock, 0, len(message.ToolCalls)+1)
		if content != "" {
			contentBlocks = append(contentBlocks, CluadeContentBlock{
				Type: "text",
				Text: content,
			})
		}
		for _, toolCall := range message.ToolCalls {
			contentBlocks = append(contentBlocks, CluadeContentBlock{
				Type:  "tool_use",
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: toolCall.Function.Arguments,
			})
		}
		requestData.Messages = append(requestData.Messages, CluadeMessage{
			Role:    "assistant",
			Content: contentBlocks,
		})
		i++
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
func (c *Cluade) ChatRaw(ctx context.Context, request requestprotocol.Request) ([]byte, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			ctx,
			c.options.Timeout,
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

	resp, err := c.options.HTTPClient.Do(req)
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

func (c *Cluade) DecodeResponse(raw []byte) (responseprotocol.Response, error) {
	type messageResponse struct {
		ID      string `json:"id"`
		Role    string `json:"role"`
		Model   string `json:"model"`
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	var decoded messageResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: "anthropic",
			Kind:     sdkerror.Decode,
			Message:  "decode response failed",
			Cause:    err,
		}
	}

	var content strings.Builder
	message := responseprotocol.Message{Role: decoded.Role}
	for _, block := range decoded.Content {
		switch block.Type {
		case "text":
			content.WriteString(block.Text)
		case "tool_use":
			message.ToolCalls = append(message.ToolCalls, responseprotocol.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: responseprotocol.FunctionCall{
					Name:      block.Name,
					Arguments: append(json.RawMessage(nil), block.Input...),
				},
			})
		}
	}
	message.Content = content.String()

	response := responseprotocol.Response{
		Provider: "anthropic",
		ID:       decoded.ID,
		Model:    decoded.Model,
		Choices: []responseprotocol.Choice{
			{
				Index:        0,
				Message:      message,
				FinishReason: decoded.StopReason,
			},
		},
		Raw: append(json.RawMessage(nil), raw...),
	}
	if decoded.Usage != nil {
		response.Usage = &responseprotocol.Usage{
			InputTokens:  decoded.Usage.InputTokens,
			OutputTokens: decoded.Usage.OutputTokens,
			TotalTokens:  decoded.Usage.InputTokens + decoded.Usage.OutputTokens,
		}
	}

	return response, nil
}
