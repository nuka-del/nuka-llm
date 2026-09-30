package responseprotocol

import "encoding/json"

type Response struct {
	Provider    string          `json:"provider,omitempty"`
	ID          string          `json:"id,omitempty"`
	Model       string          `json:"model,omitempty"`
	Message     Message         `json:"message,omitempty"`
	Choices     []Choice        `json:"choices,omitempty"`
	Usage       *Usage          `json:"usage,omitempty"`
	RoundUsages []RoundUsage    `json:"round_usages,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type RoundUsage struct {
	Round int    `json:"round"`
	Usage *Usage `json:"usage,omitempty"`
}
