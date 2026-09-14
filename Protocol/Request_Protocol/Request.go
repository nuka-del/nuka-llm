package requestprotocol

import (
	tools "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type Message struct {
	Role    string
	Content string
}

type Request struct {
	Model       string
	Messages    []Message
	Tools       tools.ToolList
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Stream      bool
	Stop        []string
}
