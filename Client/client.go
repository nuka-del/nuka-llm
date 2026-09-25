package client

import (
	"context"
	"time"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	link "github.com/nuka-del/nuka-llm/Link"
	alibaba "github.com/nuka-del/nuka-llm/Link/series/Alibaba"
	anthropic "github.com/nuka-del/nuka-llm/Link/series/Anthropic"
	deepseek "github.com/nuka-del/nuka-llm/Link/series/DeepSeek"
	openai "github.com/nuka-del/nuka-llm/Link/series/OpenAi"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
)

type Client struct {
	link link.Link
}

func New(series string, apiKey string) (*Client, error) {
	var link link.Link
	switch series {
	case "deepseek":
		link = deepseek.New(apiKey)
	case "chatgpt":
		link = openai.New(apiKey)
	case "qwen":
		link = alibaba.New(apiKey)
	case "cluade":
		link = anthropic.New(apiKey)
	default:
		return nil, &sdkerror.SDKError{
			Provider: series,
			Kind:     sdkerror.InvalidRequest,
			Message:  "unsupported provider",
		}
	}
	return &Client{
		link: link,
	}, nil
}
func (c *Client) Chat(ctx context.Context, r requestprotocol.Request) ([]byte, error) {
	return c.link.Chat(ctx, r)
}
func (c *Client) ChatWithContext(r requestprotocol.Request) ([]byte, error) {
	requestContext, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second)
	defer cancel()
	return c.link.Chat(requestContext, r)
}
