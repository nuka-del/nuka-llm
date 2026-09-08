package client

import (
	"errors"

	link "github.com/nuka-del/nuka-llm/Link"
	deepseek "github.com/nuka-del/nuka-llm/Link/series/DeepSeek"
	toolprotocol "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type Client struct {
	link  link.Link
	tools *toolprotocol.ToolList
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
		link:  link,
		tools: toolprotocol.New(),
	}, nil
}
func (c *Client) Chat(input string, model string) ([]byte, error) {
	return c.link.Chat(input, model)
}

func (c *Client) RegisterTool(t toolprotocol.Tool) {
	c.tools.Add(t)
}
func (c *Client) ToolDefinitions() ([]byte, error) {
	return c.tools.Definitions()
}
