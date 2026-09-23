package client

import (
	"context"
	"errors"
	"time"

	link "github.com/nuka-del/nuka-llm/Link"
	deepseek "github.com/nuka-del/nuka-llm/Link/series/DeepSeek"
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
	default:
		return nil, errors.New("unsupported series")
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
