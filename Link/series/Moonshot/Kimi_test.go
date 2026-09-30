package moonshot

import (
	"encoding/json"
	"testing"

	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
)

func TestBuildRequestPreservesAssistantReasoningOnly(t *testing.T) {
	request := requestprotocol.NewRequest("kimi-k3",
		requestprotocol.Message{
			Role:             "assistant",
			Content:          "",
			ReasoningContent: "reasoning from previous turn",
		},
		requestprotocol.Message{
			Role:             "user",
			Content:          "continue",
			ReasoningContent: "must not be sent for user role",
		},
	)

	body, err := New("test-key").BuildRequest(request)
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}

	var payload struct {
		Messages []map[string]json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode request JSON: %v", err)
	}
	if len(payload.Messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(payload.Messages))
	}

	var reasoning string
	if err := json.Unmarshal(payload.Messages[0]["reasoning_content"], &reasoning); err != nil {
		t.Fatalf("decode assistant reasoning_content: %v", err)
	}
	if reasoning != "reasoning from previous turn" {
		t.Errorf("reasoning_content = %q", reasoning)
	}
	if _, exists := payload.Messages[1]["reasoning_content"]; exists {
		t.Error("user message unexpectedly contains reasoning_content")
	}
}

func TestDecodeResponsePreservesReasoningContent(t *testing.T) {
	response, err := New("test-key").DecodeResponse([]byte(`{
		"id":"chatcmpl-kimi",
		"model":"kimi-k3",
		"choices":[{
			"index":0,
			"finish_reason":"tool_calls",
			"message":{
				"role":"assistant",
				"content":"",
				"reasoning_content":"call the weather tool",
				"tool_calls":[{
					"id":"call-weather",
					"type":"function",
					"function":{"name":"get_weather","arguments":"{\"location\":\"杭州\"}"}
				}]
			}
		}],
		"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}
	}`))
	if err != nil {
		t.Fatalf("DecodeResponse() error = %v", err)
	}
	if response.Message.ReasoningContent != "call the weather tool" {
		t.Errorf("Message.ReasoningContent = %q", response.Message.ReasoningContent)
	}
	if len(response.Message.ToolCalls) != 1 || response.Message.ToolCalls[0].ID != "call-weather" {
		t.Errorf("Message.ToolCalls = %#v", response.Message.ToolCalls)
	}
}
