package client

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	linkprotocol "github.com/nuka-del/nuka-llm/Link"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	toolprotocol "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

func TestDeepSeekToolRoundTripLive(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("set DEEPSEEK_API_KEY to run the live DeepSeek round-trip")
	}

	maxTokens := 256
	request := requestprotocol.Request{
		Model: "deepseek-v4-pro",
		Messages: []requestprotocol.Message{
			{
				Role:    "user",
				Content: "请分别调用 get_weather 和 search_restaurants 两个工具，查询杭州天气并搜索西湖附近餐厅。不要猜测工具结果。",
			},
		},
		MaxTokens: &maxTokens,
		Tools: toolprotocol.Tools{
			ToolList: []toolprotocol.Tool{
				{
					ToolType: "function",
					Function: toolprotocol.Function{
						Name:        "get_weather",
						Description: "查询指定城市的当前天气。",
						Parameters: toolprotocol.Parameters{
							ParaType: "object",
							SimpleProperties: map[string]toolprotocol.Property{
								"location": {PropertiesType: "string", Description: "城市名称"},
							},
							Required: []string{"location"},
						},
					},
				},
				{
					ToolType: "function",
					Function: toolprotocol.Function{
						Name:        "search_restaurants",
						Description: "搜索指定地点附近的餐厅。",
						Parameters: toolprotocol.Parameters{
							ParaType: "object",
							SimpleProperties: map[string]toolprotocol.Property{
								"city": {PropertiesType: "string", Description: "城市名称"},
								"near": {PropertiesType: "string", Description: "附近地点"},
							},
							Required: []string{"city", "near"},
						},
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	sdk, err := NewWithOptions("deepseek", apiKey, linkprotocol.Options{Timeout: 90 * time.Second})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	firstResponse, err := sdk.Chat(ctx, request)
	if err != nil {
		t.Fatalf("first DeepSeek request failed: %v", err)
	}
	firstJSON, _ := json.MarshalIndent(firstResponse, "", "  ")
	t.Logf("first normalized response: %s", firstJSON)

	if len(firstResponse.Choices) == 0 {
		t.Fatal("first response contains no choices")
	}
	assistantMessage := firstResponse.Choices[0].Message
	if len(assistantMessage.ToolCalls) != 2 {
		t.Fatalf("DeepSeek returned %d tool calls, want 2", len(assistantMessage.ToolCalls))
	}

	toolResults := map[string]any{}
	for _, call := range assistantMessage.ToolCalls {
		switch call.Function.Name {
		case "get_weather":
			toolResults[call.ID] = map[string]any{
				"location":      "杭州",
				"weather":       "晴",
				"temperature_c": 26,
			}
		case "search_restaurants":
			toolResults[call.ID] = map[string]any{
				"city":        "杭州",
				"near":        "西湖",
				"restaurants": []string{"湖畔餐厅", "山外山"},
			}
		default:
			t.Fatalf("unexpected tool call: %s", call.Function.Name)
		}
	}

	nextRequest := request
	nextRequest.Messages = append([]requestprotocol.Message(nil), request.Messages...)
	nextRequest.Messages = append(nextRequest.Messages, requestprotocol.Message{
		Role:      assistantMessage.Role,
		Content:   assistantMessage.Content,
		ToolCalls: assistantMessage.ToolCalls,
	})
	for _, call := range assistantMessage.ToolCalls {
		nextRequest.Messages = append(nextRequest.Messages, requestprotocol.Message{
			Role:       "tool",
			ToolCallID: call.ID,
			Content:    toolResults[call.ID],
		})
	}

	secondResponse, err := sdk.Chat(ctx, nextRequest)
	if err != nil {
		t.Fatalf("second DeepSeek request with tool results failed: %v", err)
	}
	secondJSON, _ := json.MarshalIndent(secondResponse, "", "  ")
	t.Logf("second normalized response: %s", secondJSON)
	if len(secondResponse.Choices) == 0 {
		t.Fatal("second response contains no choices")
	}
	t.Logf("final answer: %s", secondResponse.Choices[0].Message.Content)
}
