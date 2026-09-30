package toolprotocol

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
)

type ToolResult struct {
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

type ToolExecutor interface {
	Name() string
	Start(arguments json.RawMessage) (ToolResult, error)
}

type ToolRegistry struct {
	tools   map[string]ToolExecutor
	results []ToolResult
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolExecutor),
	}
}

func (r *ToolRegistry) Register(tool ToolExecutor) error {
	if r == nil {
		return &sdkerror.SDKError{
			Kind:    sdkerror.InvalidRequest,
			Message: "tool registry is not initialized",
		}
	}
	if isNilExecutor(tool) {
		return &sdkerror.SDKError{
			Kind:    sdkerror.InvalidRequest,
			Message: "tool executor is nil",
		}
	}

	name := tool.Name()
	if strings.TrimSpace(name) == "" {
		return &sdkerror.SDKError{
			Kind:    sdkerror.InvalidRequest,
			Message: "tool name is required",
		}
	}
	if r.tools == nil {
		r.tools = make(map[string]ToolExecutor)
	}
	if _, exists := r.tools[name]; exists {
		return &sdkerror.SDKError{
			Kind:    sdkerror.InvalidRequest,
			Message: fmt.Sprintf("tool %q is already registered", name),
		}
	}

	r.tools[name] = tool
	return nil
}

func (r *ToolRegistry) Dispatch(response responseprotocol.Response) error {
	if r == nil {
		return &sdkerror.SDKError{
			Provider: response.Provider,
			Kind:     sdkerror.InvalidRequest,
			Message:  "tool registry is not initialized",
		}
	}
	r.results = nil
	dispatched := 0
	for _, choice := range response.Choices {
		for _, toolCall := range choice.Message.ToolCalls {
			name := toolCall.Function.Name
			arguments := toolCall.Function.Arguments

			if strings.TrimSpace(toolCall.ID) == "" {
				r.results = nil
				return &sdkerror.SDKError{
					Provider: response.Provider,
					Kind:     sdkerror.InvalidRequest,
					Message:  fmt.Sprintf("tool call ID is required for tool %q", name),
				}
			}
			if strings.TrimSpace(name) == "" {
				r.results = nil
				return &sdkerror.SDKError{
					Provider: response.Provider,
					Kind:     sdkerror.InvalidRequest,
					Message:  "tool function name is required",
				}
			}

			executor, exists := r.tools[name]
			if !exists || isNilExecutor(executor) {
				r.results = nil
				return &sdkerror.SDKError{
					Provider: response.Provider,
					Kind:     sdkerror.InvalidRequest,
					Message:  fmt.Sprintf("tool %q is not registered", name),
				}
			}

			result, err := startTool(executor, arguments)
			if err != nil {
				r.results = nil
				return &sdkerror.SDKError{
					Provider: response.Provider,
					Kind:     sdkerror.ToolExecution,
					Message:  fmt.Sprintf("tool %q execution failed", name),
					Cause:    err,
				}
			}
			if strings.TrimSpace(result.Content) == "" {
				r.results = nil
				return &sdkerror.SDKError{
					Provider: response.Provider,
					Kind:     sdkerror.ToolExecution,
					Message:  fmt.Sprintf("tool %q returned an empty result", name),
				}
			}
			result.ToolCallID = toolCall.ID

			r.results = append(r.results, result)
			dispatched++
		}
	}
	if dispatched == 0 {
		return &sdkerror.SDKError{
			Provider: response.Provider,
			Kind:     sdkerror.InvalidRequest,
			Message:  "response contains no tool calls",
		}
	}
	return nil
}

func (r *ToolRegistry) Results() []ToolResult {
	if r == nil {
		return nil
	}
	return append([]ToolResult(nil), r.results...)
}

func isNilExecutor(executor ToolExecutor) bool {
	if executor == nil {
		return true
	}

	value := reflect.ValueOf(executor)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func startTool(executor ToolExecutor, arguments json.RawMessage) (result ToolResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("tool executor panicked: %v", recovered)
		}
	}()
	return executor.Start(arguments)
}
