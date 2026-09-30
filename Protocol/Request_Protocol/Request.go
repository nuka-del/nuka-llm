package requestprotocol

import (
	"encoding/json"

	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
	tools "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type Message struct {
	Role       string
	Content    any
	ToolCalls  []responseprotocol.ToolCall
	ToolCallID string
}

type Request struct {
	Model       string
	Messages    []Message
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Stream      bool
	Stop        []string
	Tools       tools.Tools
}

// ContentText returns text unchanged and JSON-encodes non-string message content.
func ContentText(content any) (string, error) {
	if text, ok := content.(string); ok {
		return text, nil
	}
	if content == nil {
		return "", nil
	}

	data, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
