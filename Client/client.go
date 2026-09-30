package client

import (
	"context"
	"strings"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	link "github.com/nuka-del/nuka-llm/Link"
	alibaba "github.com/nuka-del/nuka-llm/Link/series/Alibaba"
	anthropic "github.com/nuka-del/nuka-llm/Link/series/Anthropic"
	deepseek "github.com/nuka-del/nuka-llm/Link/series/DeepSeek"
	moonshot "github.com/nuka-del/nuka-llm/Link/series/Moonshot"
	openai "github.com/nuka-del/nuka-llm/Link/series/OpenAi"
	zhipuai "github.com/nuka-del/nuka-llm/Link/series/ZhipuAI"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
	toolprotocol "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type Client struct {
	link link.Link
	ctx  context.Context
}

func New(series string, apiKey string) (*Client, error) {
	return NewWithOptions(series, apiKey, link.DefaultOptions())
}

func NewWithOptions(series string, apiKey string, options link.Options) (*Client, error) {
	options = options.WithDefaults()
	var providerLink link.Link
	switch series {
	case "deepseek":
		providerLink = deepseek.NewWithOptions(apiKey, options)
	case "chatgpt":
		providerLink = openai.NewWithOptions(apiKey, options)
	case "qwen":
		providerLink = alibaba.NewWithOptions(apiKey, options)
	case "glm":
		providerLink = zhipuai.NewWithOptions(apiKey, options)
	case "kimi":
		providerLink = moonshot.NewWithOptions(apiKey, options)
	case "cluade":
		providerLink = anthropic.NewWithOptions(apiKey, options)
	default:
		return nil, &sdkerror.SDKError{
			Provider: series,
			Kind:     sdkerror.InvalidRequest,
			Message:  "unsupported provider",
		}
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, &sdkerror.SDKError{
			Provider: series,
			Kind:     sdkerror.InvalidRequest,
			Message:  "API key is required",
		}
	}
	return &Client{
		link: providerLink,
	}, nil
}
func (c *Client) Chat(r requestprotocol.Request) (responseprotocol.Response, error) {
	return c.chat(c.requestContext(), r)
}

func (c *Client) chat(
	ctx context.Context,
	r requestprotocol.Request,
) (responseprotocol.Response, error) {
	if c == nil || c.link == nil {
		return responseprotocol.Response{}, &sdkerror.SDKError{
			Kind:    sdkerror.InvalidRequest,
			Message: "client is not initialized",
		}
	}
	raw, err := c.link.ChatRaw(ctx, r)
	if err != nil {
		return responseprotocol.Response{}, err
	}
	response, err := c.link.DecodeResponse(raw)
	if err != nil {
		return responseprotocol.Response{}, err
	}
	if len(response.Choices) > 0 {
		response.Message = response.Choices[0].Message
	}
	response.RoundUsages = []responseprotocol.RoundUsage{
		{
			Round: 1,
			Usage: copyUsage(response.Usage),
		},
	}
	return response, nil
}
func (c *Client) ChatRaw(r requestprotocol.Request) ([]byte, error) {
	if c == nil || c.link == nil {
		return nil, &sdkerror.SDKError{
			Kind:    sdkerror.InvalidRequest,
			Message: "client is not initialized",
		}
	}
	return c.link.ChatRaw(c.requestContext(), r)
}

func (c *Client) RegisterTool(executor toolprotocol.ToolExecutor) error {
	if c == nil || c.link == nil {
		return &sdkerror.SDKError{
			Kind:    sdkerror.InvalidRequest,
			Message: "client is not initialized",
		}
	}
	return c.link.RegisterTool(executor)
}

func (c *Client) SetContext(ctx context.Context) error {
	if c == nil {
		return &sdkerror.SDKError{
			Kind:    sdkerror.InvalidRequest,
			Message: "client is not initialized",
		}
	}
	c.ctx = ctx
	return nil
}

func (c *Client) ChatWithTools(
	r requestprotocol.Request,
) (responseprotocol.Response, error) {
	ctx := c.requestContext()
	response, err := c.chat(ctx, r)
	if err != nil {
		return responseprotocol.Response{}, err
	}

	var totalUsage responseprotocol.Usage
	hasUsage := false
	roundUsages := make([]responseprotocol.RoundUsage, 0, 1)
	appendRoundUsage := func(round int, usage *responseprotocol.Usage) {
		copiedUsage := copyUsage(usage)
		roundUsages = append(roundUsages, responseprotocol.RoundUsage{
			Round: round,
			Usage: copiedUsage,
		})
		if copiedUsage == nil {
			return
		}
		hasUsage = true
		totalUsage.InputTokens += copiedUsage.InputTokens
		totalUsage.OutputTokens += copiedUsage.OutputTokens
		totalUsage.TotalTokens += copiedUsage.TotalTokens
	}

	round := 1
	appendRoundUsage(round, response.Usage)
	for len(response.Message.ToolCalls) > 0 {
		response, err = c.link.ChatWithToolResults(ctx, &r, response)
		if err != nil {
			return responseprotocol.Response{}, err
		}
		if len(response.Choices) > 0 {
			response.Message = response.Choices[0].Message
		}
		round++
		appendRoundUsage(round, response.Usage)
	}

	response.RoundUsages = roundUsages
	if hasUsage {
		response.Usage = &totalUsage
	} else {
		response.Usage = nil
	}
	return response, nil
}

func copyUsage(usage *responseprotocol.Usage) *responseprotocol.Usage {
	if usage == nil {
		return nil
	}
	copied := *usage
	return &copied
}

func (c *Client) requestContext() context.Context {
	if c != nil && c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}
