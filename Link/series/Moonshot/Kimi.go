package moonshot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	link "github.com/nuka-del/nuka-llm/Link"
	openai "github.com/nuka-del/nuka-llm/Link/series/OpenAi"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
	toolprotocol "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
	tool "github.com/nuka-del/nuka-llm/Tool"
)

const providerName = "moonshot"
const chatCompletionsURL = "https://api.moonshot.cn/v1/chat/completions"

type Kimi struct {
	apiKey       string
	options      link.Options
	mapper       *openai.Chatgpt
	toolRegistry *toolprotocol.ToolRegistry
}

func New(apiKey string) *Kimi {
	return NewWithOptions(apiKey, link.DefaultOptions())
}

func NewWithOptions(apiKey string, options link.Options) *Kimi {
	options = options.WithDefaults()
	return &Kimi{
		apiKey:       apiKey,
		options:      options,
		mapper:       openai.NewWithOptions(apiKey, options),
		toolRegistry: toolprotocol.NewToolRegistry(),
	}
}

func (m *Kimi) RegisterTool(executor toolprotocol.ToolExecutor) error {
	if m == nil || m.toolRegistry == nil {
		return &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "tool registry is not initialized",
		}
	}
	if err := m.toolRegistry.Register(executor); err != nil {
		return m.withProvider(err)
	}
	return nil
}

func (m *Kimi) ChatWithToolResults(
	ctx context.Context,
	request *requestprotocol.Request,
	firstResponse responseprotocol.Response,
) (responseprotocol.Response, error) {
	if m == nil || m.toolRegistry == nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "tool registry is not initialized",
		}
	}
	if ctx == nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "context must not be nil",
		}
	}
	if request == nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "request must not be nil",
		}
	}
	if strings.TrimSpace(m.apiKey) == "" {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "API key is required",
		}
	}
	if m.options.HTTPClient == nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "HTTP client is not initialized",
		}
	}
	if len(firstResponse.Choices) == 0 {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "first response contains no choices",
		}
	}

	toolResponse := firstResponse
	toolResponse.Choices = toolResponse.Choices[:1]
	if err := m.toolRegistry.Dispatch(toolResponse); err != nil {
		return responseprotocol.Response{}, m.withProvider(err)
	}
	results := m.toolRegistry.Results()
	if len(results) == 0 {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "tool dispatch returned no results",
		}
	}

	assistantMessage := firstResponse.Choices[0].Message
	request.Messages = append(request.Messages,
		requestprotocol.Message{
			Role:             assistantMessage.Role,
			Content:          assistantMessage.Content,
			ReasoningContent: assistantMessage.ReasoningContent,
			ToolCalls:        assistantMessage.ToolCalls,
		},
	)
	for _, result := range results {
		request.Messages = append(request.Messages,
			requestprotocol.Message{
				Role:       "tool",
				ToolCallID: result.ToolCallID,
				Content:    result.Content,
			},
		)
	}

	raw, err := m.ChatRaw(ctx, *request)
	if err != nil {
		return responseprotocol.Response{}, err
	}
	return m.DecodeResponse(raw)
}

func (m *Kimi) BuildRequest(request requestprotocol.Request) ([]byte, error) {
	if m == nil || m.mapper == nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "provider is not initialized",
		}
	}
	body, err := m.mapper.BuildRequest(request)
	if err != nil {
		return nil, m.withProvider(err)
	}
	hasReasoningContent := false
	for _, message := range request.Messages {
		if message.Role == "assistant" && message.ReasoningContent != "" {
			hasReasoningContent = true
			break
		}
	}
	if !hasReasoningContent {
		return body, nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "decode mapped request failed",
			Cause:    err,
		}
	}
	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(payload["messages"], &messages); err != nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "decode mapped messages failed",
			Cause:    err,
		}
	}
	if len(messages) != len(request.Messages) {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "mapped message count does not match request",
		}
	}
	for i, message := range request.Messages {
		if message.Role != "assistant" || message.ReasoningContent == "" {
			continue
		}
		reasoningContent, err := json.Marshal(message.ReasoningContent)
		if err != nil {
			return nil, &sdkerror.SDKError{
				Provider: providerName,
				Kind:     sdkerror.InvalidRequest,
				Message:  "encode reasoning content failed",
				Cause:    err,
			}
		}
		messages[i]["reasoning_content"] = reasoningContent
	}
	encodedMessages, err := json.Marshal(messages)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "encode mapped messages failed",
			Cause:    err,
		}
	}
	payload["messages"] = encodedMessages
	body, err = json.Marshal(payload)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "encode mapped request failed",
			Cause:    err,
		}
	}
	return body, nil
}

func (m *Kimi) ChatRaw(ctx context.Context, request requestprotocol.Request) ([]byte, error) {
	if m == nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "provider is not initialized",
		}
	}
	if ctx == nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "context must not be nil",
		}
	}
	if strings.TrimSpace(m.apiKey) == "" {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "API key is required",
		}
	}
	if m.options.HTTPClient == nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "HTTP client is not initialized",
		}
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && m.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.options.Timeout)
		defer cancel()
	}

	jsonData, err := m.BuildRequest(request)
	if err != nil {
		return nil, err
	}
	factory := &tool.HttpFactory{}
	factory.Set(chatCompletionsURL, m.apiKey)
	requestMessage, err := factory.Create(jsonData)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "create HTTP request failed",
			Cause:    err,
		}
	}
	requestMessage = requestMessage.WithContext(ctx)

	response, err := m.options.HTTPClient.Do(requestMessage)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.Transport,
			Message:  "send HTTP request failed",
			Cause:    err,
		}
	}
	defer response.Body.Close()

	result, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.Transport,
			Message:  "read response body failed",
			Cause:    err,
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &sdkerror.SDKError{
			Provider:   providerName,
			Kind:       sdkerror.API,
			StatusCode: response.StatusCode,
			Message:    "request returned unsuccessful status",
			Body:       string(result),
		}
	}
	return result, nil
}

func (m *Kimi) DecodeResponse(raw []byte) (responseprotocol.Response, error) {
	if m == nil || m.mapper == nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "provider is not initialized",
		}
	}
	response, err := m.mapper.DecodeResponse(raw)
	if err != nil {
		return responseprotocol.Response{}, m.withProvider(err)
	}
	var kimiResponse struct {
		Choices []struct {
			Message struct {
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &kimiResponse); err != nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.Decode,
			Message:  "decode reasoning content failed",
			Cause:    err,
		}
	}
	for i := range response.Choices {
		if i < len(kimiResponse.Choices) {
			response.Choices[i].Message.ReasoningContent = kimiResponse.Choices[i].Message.ReasoningContent
		}
	}
	if len(response.Choices) > 0 {
		response.Message = response.Choices[0].Message
	}
	response.Provider = providerName
	return response, nil
}

func (m *Kimi) withProvider(err error) error {
	var providerError *sdkerror.SDKError
	if errors.As(err, &providerError) {
		clonedError := *providerError
		clonedError.Provider = providerName
		return &clonedError
	}
	return err
}
