package toolprotocol

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
)

type testToolExecutor struct {
	name       string
	result     ToolResult
	startErr   error
	panicValue any
}

func (t testToolExecutor) Name() string {
	return t.name
}

func (t testToolExecutor) Start(json.RawMessage) (ToolResult, error) {
	if t.panicValue != nil {
		panic(t.panicValue)
	}
	return t.result, t.startErr
}

func TestDispatchReturnsErrorForUnregisteredTool(t *testing.T) {
	registry := NewToolRegistry()
	err := registry.Dispatch(responseprotocol.Response{
		Choices: []responseprotocol.Choice{{
			Message: responseprotocol.Message{
				ToolCalls: []responseprotocol.ToolCall{{
					ID:   "call_weather",
					Type: "function",
					Function: responseprotocol.FunctionCall{
						Name:      "get_weather",
						Arguments: json.RawMessage(`{"location":"杭州"}`),
					},
				}},
			},
		}},
	})

	if err == nil {
		t.Fatal("Dispatch() error = nil, want unregistered tool error")
	}
	var sdkErr *sdkerror.SDKError
	if !errors.As(err, &sdkErr) {
		t.Fatalf("Dispatch() error type = %T, want *SDKError", err)
	}
	if sdkErr.Kind != sdkerror.InvalidRequest {
		t.Errorf("error kind = %q, want %q", sdkErr.Kind, sdkerror.InvalidRequest)
	}
	if !strings.Contains(sdkErr.Message, `tool "get_weather" is not registered`) {
		t.Errorf("error message = %q", sdkErr.Message)
	}
}

func TestDispatchReturnsErrorForEmptyToolResult(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(testToolExecutor{name: "get_weather"})

	err := registry.Dispatch(responseprotocol.Response{
		Choices: []responseprotocol.Choice{{
			Message: responseprotocol.Message{
				ToolCalls: []responseprotocol.ToolCall{{
					ID:   "call_weather",
					Type: "function",
					Function: responseprotocol.FunctionCall{
						Name:      "get_weather",
						Arguments: json.RawMessage(`{"location":"杭州"}`),
					},
				}},
			},
		}},
	})

	if err == nil {
		t.Fatal("Dispatch() error = nil, want empty tool result error")
	}
	var sdkErr *sdkerror.SDKError
	if !errors.As(err, &sdkErr) {
		t.Fatalf("Dispatch() error type = %T, want *SDKError", err)
	}
	if !strings.Contains(sdkErr.Message, `tool "get_weather" returned an empty result`) {
		t.Errorf("error message = %q", sdkErr.Message)
	}
	if results := registry.Results(); len(results) != 0 {
		t.Errorf("Results() = %#v, want no partial results", results)
	}
}

func TestDispatchSavesToolCallIDOnResult(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(testToolExecutor{
		name: "get_weather",
		result: ToolResult{
			Content: `{"weather":"晴"}`,
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := registry.Dispatch(responseprotocol.Response{
		Choices: []responseprotocol.Choice{{
			Message: responseprotocol.Message{
				ToolCalls: []responseprotocol.ToolCall{{
					ID:   "call_weather",
					Type: "function",
					Function: responseprotocol.FunctionCall{
						Name:      "get_weather",
						Arguments: json.RawMessage(`{"location":"杭州"}`),
					},
				}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	results := registry.Results()
	if len(results) != 1 {
		t.Fatalf("Results() length = %d, want 1", len(results))
	}
	if results[0].ToolCallID != "call_weather" {
		t.Errorf("ToolCallID = %q, want call_weather", results[0].ToolCallID)
	}
	if results[0].Content != `{"weather":"晴"}` {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestRegisterRejectsNilAndDuplicateExecutors(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(nil); err == nil {
		t.Fatal("Register(nil) error = nil, want error")
	}

	executor := testToolExecutor{name: "get_weather"}
	if err := registry.Register(executor); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := registry.Register(executor); err == nil {
		t.Fatal("duplicate Register() error = nil, want error")
	}
}

func TestDispatchRejectsMissingToolCallID(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(testToolExecutor{
		name:   "get_weather",
		result: ToolResult{Content: `{"weather":"晴"}`},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := registry.Dispatch(responseprotocol.Response{
		Provider: "deepseek",
		Choices: []responseprotocol.Choice{{
			Message: responseprotocol.Message{
				ToolCalls: []responseprotocol.ToolCall{{
					Function: responseprotocol.FunctionCall{
						Name:      "get_weather",
						Arguments: json.RawMessage(`{"location":"杭州"}`),
					},
				}},
			},
		}},
	})
	if err == nil {
		t.Fatal("Dispatch() error = nil, want missing ID error")
	}
	var sdkErr *sdkerror.SDKError
	if !errors.As(err, &sdkErr) || sdkErr.Kind != sdkerror.InvalidRequest {
		t.Fatalf("Dispatch() error = %v, want invalid request SDKError", err)
	}
	if !strings.Contains(sdkErr.Message, "tool call ID is required") {
		t.Errorf("error message = %q", sdkErr.Message)
	}
}

func TestDispatchReturnsToolExecutionError(t *testing.T) {
	startErr := errors.New("backend unavailable")
	registry := NewToolRegistry()
	if err := registry.Register(testToolExecutor{
		name:     "get_weather",
		startErr: startErr,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := registry.Dispatch(responseprotocol.Response{
		Provider: "deepseek",
		Choices: []responseprotocol.Choice{{
			Message: responseprotocol.Message{
				ToolCalls: []responseprotocol.ToolCall{{
					ID: "call_weather",
					Function: responseprotocol.FunctionCall{
						Name:      "get_weather",
						Arguments: json.RawMessage(`{"location":"杭州"}`),
					},
				}},
			},
		}},
	})
	if err == nil {
		t.Fatal("Dispatch() error = nil, want tool execution error")
	}
	var sdkErr *sdkerror.SDKError
	if !errors.As(err, &sdkErr) || sdkErr.Kind != sdkerror.ToolExecution {
		t.Fatalf("Dispatch() error = %v, want tool execution SDKError", err)
	}
	if !errors.Is(err, startErr) {
		t.Errorf("Dispatch() error = %v, want wrapped Start() error", err)
	}
}

func TestDispatchRecoversToolPanic(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(testToolExecutor{
		name:       "get_weather",
		panicValue: "unexpected failure",
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := registry.Dispatch(responseprotocol.Response{
		Choices: []responseprotocol.Choice{{
			Message: responseprotocol.Message{
				ToolCalls: []responseprotocol.ToolCall{{
					ID: "call_weather",
					Function: responseprotocol.FunctionCall{
						Name: "get_weather",
					},
				}},
			},
		}},
	})
	if err == nil {
		t.Fatal("Dispatch() error = nil, want recovered panic error")
	}
	var sdkErr *sdkerror.SDKError
	if !errors.As(err, &sdkErr) || sdkErr.Kind != sdkerror.ToolExecution {
		t.Fatalf("Dispatch() error = %v, want tool execution SDKError", err)
	}
}
