package client

import (
	"context"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	link "github.com/nuka-del/nuka-llm/Link"
	alibaba "github.com/nuka-del/nuka-llm/Link/series/Alibaba"
	anthropic "github.com/nuka-del/nuka-llm/Link/series/Anthropic"
	deepseek "github.com/nuka-del/nuka-llm/Link/series/DeepSeek"
	openai "github.com/nuka-del/nuka-llm/Link/series/OpenAi"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
)

type Client struct {
	link    link.Link
	options link.Options
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
	case "cluade":
		providerLink = anthropic.NewWithOptions(apiKey, options)
	default:
		return nil, &sdkerror.SDKError{
			Provider: series,
			Kind:     sdkerror.InvalidRequest,
			Message:  "unsupported provider",
		}
	}
	return &Client{
		link:    providerLink,
		options: options,
	}, nil
}
func (c *Client) Chat(ctx context.Context, r requestprotocol.Request) (responseprotocol.Response, error) {
	raw, err := c.ChatRaw(ctx, r)
	if err != nil {
		return responseprotocol.Response{}, err
	}
	return c.link.DecodeResponse(raw)
}
func (c *Client) ChatRaw(ctx context.Context, r requestprotocol.Request) ([]byte, error) {
	return c.link.ChatRaw(ctx, r)
}
func (c *Client) ChatWithContext(r requestprotocol.Request) (responseprotocol.Response, error) {
	requestContext, cancel := context.WithTimeout(
		context.Background(),
		c.options.Timeout)
	defer cancel()
	return c.Chat(requestContext, r)
}
