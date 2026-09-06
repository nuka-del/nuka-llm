package client

import (
	"errors"

	link "github.com/nuka-del/nuka-llm/Link"
	deepseek "github.com/nuka-del/nuka-llm/Link/series/DeepSeek"
)

type Client struct {
	link link.Link
}

func New(series string, apikey string) (*Client, error) {
	var link link.Link
	switch series {
	case "deepseek":
		link = deepseek.New(apikey)
	default:
		return nil, errors.New("unsupported series")
	}
	return &Client{
		link: link,
	}, nil
}
func (c *Client) Chat(input string, model string) ([]byte, error) {
	return c.link.Chat(input, model)
}
