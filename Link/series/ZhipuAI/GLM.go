package zhipuai

import (
	"context"
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

const providerName = "glm"
const chatCompletionsURL = "https://open.bigmodel.cn/api/paas/v4/chat/completions"

type GLM struct {
	apiKey       string
	options      link.Options
	mapper       *openai.Chatgpt
	toolRegistry *toolprotocol.ToolRegistry
}

func New(apiKey string) *GLM {
	return NewWithOptions(apiKey, link.DefaultOptions())
}

func NewWithOptions(apiKey string, options link.Options) *GLM {
	options = options.WithDefaults()
	return &GLM{
		apiKey:       apiKey,
		options:      options,
		mapper:       openai.NewWithOptions(apiKey, options),
		toolRegistry: toolprotocol.NewToolRegistry(),
	}
}

func (g *GLM) RegisterTool(executor toolprotocol.ToolExecutor) error {
	if g == nil || g.toolRegistry == nil {
		return &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "tool registry is not initialized",
		}
	}
	if err := g.toolRegistry.Register(executor); err != nil {
		return g.withProvider(err)
	}
	return nil
}

func (g *GLM) ChatWithToolResults(
	ctx context.Context,
	request *requestprotocol.Request,
	firstResponse responseprotocol.Response,
) (responseprotocol.Response, error) {
	if g == nil || g.toolRegistry == nil {
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
	if strings.TrimSpace(g.apiKey) == "" {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "API key is required",
		}
	}
	if g.options.HTTPClient == nil {
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
	if err := g.toolRegistry.Dispatch(toolResponse); err != nil {
		return responseprotocol.Response{}, g.withProvider(err)
	}
	results := g.toolRegistry.Results()
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
			Role:      assistantMessage.Role,
			Content:   assistantMessage.Content,
			ToolCalls: assistantMessage.ToolCalls,
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

	raw, err := g.ChatRaw(ctx, *request)
	if err != nil {
		return responseprotocol.Response{}, err
	}
	return g.DecodeResponse(raw)
}

func (g *GLM) BuildRequest(request requestprotocol.Request) ([]byte, error) {
	if g == nil || g.mapper == nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "provider is not initialized",
		}
	}
	body, err := g.mapper.BuildRequest(request)
	if err != nil {
		return nil, g.withProvider(err)
	}
	return body, nil
}

func (g *GLM) ChatRaw(ctx context.Context, request requestprotocol.Request) ([]byte, error) {
	if g == nil {
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
	if strings.TrimSpace(g.apiKey) == "" {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "API key is required",
		}
	}
	if g.options.HTTPClient == nil {
		return nil, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "HTTP client is not initialized",
		}
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && g.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.options.Timeout)
		defer cancel()
	}

	jsonData, err := g.BuildRequest(request)
	if err != nil {
		return nil, err
	}
	factory := &tool.HttpFactory{}
	factory.Set(chatCompletionsURL, g.apiKey)
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

	response, err := g.options.HTTPClient.Do(requestMessage)
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

func (g *GLM) DecodeResponse(raw []byte) (responseprotocol.Response, error) {
	if g == nil || g.mapper == nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Provider: providerName,
			Kind:     sdkerror.InvalidRequest,
			Message:  "provider is not initialized",
		}
	}
	response, err := g.mapper.DecodeResponse(raw)
	if err != nil {
		return responseprotocol.Response{}, g.withProvider(err)
	}
	response.Provider = providerName
	return response, nil
}

func (g *GLM) withProvider(err error) error {
	var providerError *sdkerror.SDKError
	if errors.As(err, &providerError) {
		clonedError := *providerError
		clonedError.Provider = providerName
		return &clonedError
	}
	return err
}
