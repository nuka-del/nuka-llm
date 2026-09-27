package responseprotocol_test

import (
	"encoding/json"
	"errors"
	"testing"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	alibaba "github.com/nuka-del/nuka-llm/Link/series/Alibaba"
	anthropic "github.com/nuka-del/nuka-llm/Link/series/Anthropic"
	deepseek "github.com/nuka-del/nuka-llm/Link/series/DeepSeek"
	openai "github.com/nuka-del/nuka-llm/Link/series/OpenAi"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
)

func TestProviderDecodeResponse(t *testing.T) {
	chatCompletionResponse := []byte(`{
		"id":"chatcmpl-test",
		"model":"test-model",
		"choices":[{
			"index":0,
			"finish_reason":"tool_calls",
			"message":{
				"role":"assistant",
				"content":"正在查询",
				"tool_calls":[{
					"id":"call-test",
					"type":"function",
					"function":{
						"name":"get_weather",
						"arguments":"{\"location\":\"杭州\"}"
					}
				}]
			}
		}],
		"usage":{"prompt_tokens":10,"completion_tokens":6,"total_tokens":16}
	}`)

	claudeResponse := []byte(`{
		"id":"msg-test",
		"role":"assistant",
		"model":"test-model",
		"content":[
			{"type":"text","text":"正在查询"},
			{"type":"tool_use","id":"toolu-test","name":"get_weather","input":{"location":"杭州"}}
		],
		"stop_reason":"tool_use",
		"usage":{"input_tokens":10,"output_tokens":6}
	}`)

	testCases := []struct {
		name     string
		provider string
		raw      []byte
		decode   func([]byte) (responseprotocol.Response, error)
	}{
		{
			name:     "OpenAI",
			provider: "openai",
			raw:      chatCompletionResponse,
			decode:   openai.New("").DecodeResponse,
		},
		{
			name:     "DeepSeek",
			provider: "deepseek",
			raw:      chatCompletionResponse,
			decode:   deepseek.New("").DecodeResponse,
		},
		{
			name:     "Qwen",
			provider: "qwen",
			raw:      chatCompletionResponse,
			decode:   alibaba.New("").DecodeResponse,
		},
		{
			name:     "Anthropic",
			provider: "anthropic",
			raw:      claudeResponse,
			decode:   anthropic.New("").DecodeResponse,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.decode(testCase.raw)
			if err != nil {
				t.Fatalf("DecodeResponse() error = %v", err)
			}
			if got.Provider != testCase.provider {
				t.Errorf("Provider = %q, want %q", got.Provider, testCase.provider)
			}
			if got.ID == "" || got.Model != "test-model" {
				t.Errorf("ID/Model = %q/%q", got.ID, got.Model)
			}
			if len(got.Choices) != 1 {
				t.Fatalf("Choices length = %d, want 1", len(got.Choices))
			}
			choice := got.Choices[0]
			if choice.FinishReason != "tool_calls" && choice.FinishReason != "tool_use" {
				t.Errorf("FinishReason = %q", choice.FinishReason)
			}
			if choice.Message.Content != "正在查询" {
				t.Errorf("Content = %q", choice.Message.Content)
			}
			if len(choice.Message.ToolCalls) != 1 {
				t.Fatalf("ToolCalls length = %d, want 1", len(choice.Message.ToolCalls))
			}
			toolCall := choice.Message.ToolCalls[0]
			if toolCall.Function.Name != "get_weather" {
				t.Errorf("Function.Name = %q", toolCall.Function.Name)
			}
			if !json.Valid(toolCall.Function.Arguments) {
				t.Fatalf("Arguments are not valid JSON: %s", toolCall.Function.Arguments)
			}
			if got.Usage == nil || got.Usage.InputTokens != 10 || got.Usage.OutputTokens != 6 || got.Usage.TotalTokens != 16 {
				t.Errorf("Usage = %#v", got.Usage)
			}
			if string(got.Raw) != string(testCase.raw) {
				t.Errorf("Raw response was not preserved")
			}
		})
	}
}

func TestDecodeResponseReturnsSDKDecodeError(t *testing.T) {
	_, err := openai.New("").DecodeResponse([]byte(`{"choices":[`))
	if err == nil {
		t.Fatal("DecodeResponse() error = nil, want decode error")
	}

	var sdkErr *sdkerror.SDKError
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *SDKError", err)
	}
	if sdkErr.Kind != sdkerror.Decode {
		t.Errorf("error kind = %q, want %q", sdkErr.Kind, sdkerror.Decode)
	}
}
