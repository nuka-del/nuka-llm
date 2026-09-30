package client

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	linkprotocol "github.com/nuka-del/nuka-llm/Link"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	toolprotocol "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type liveToolExecutor struct {
	name  string
	calls int
}

func (t *liveToolExecutor) Name() string {
	return t.name
}

func (t *liveToolExecutor) Start(arguments json.RawMessage) (toolprotocol.ToolResult, error) {
	if !json.Valid(arguments) {
		return toolprotocol.ToolResult{}, errors.New("tool arguments are not valid JSON")
	}
	t.calls++
	var result map[string]any
	switch t.name {
	case "get_weather":
		result = map[string]any{
			"location":      "杭州",
			"weather":       "晴",
			"temperature_c": 26,
		}
	case "search_restaurants":
		result = map[string]any{
			"city":        "杭州",
			"near":        "西湖",
			"restaurants": []string{"湖畔餐厅", "山外山"},
		}
	default:
		return toolprotocol.ToolResult{}, errors.New("unknown tool")
	}
	content, err := json.Marshal(result)
	if err != nil {
		return toolprotocol.ToolResult{}, err
	}
	return toolprotocol.ToolResult{Content: string(content)}, nil
}

func TestDeepSeekLiveToolLoop(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY is not set")
	}

	sdk, err := NewWithOptions("deepseek", apiKey, linkprotocol.Options{
		Timeout: 90 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := sdk.SetContext(ctx); err != nil {
		t.Fatalf("SetContext() error = %v", err)
	}

	weatherTool := &liveToolExecutor{name: "get_weather"}
	restaurantTool := &liveToolExecutor{name: "search_restaurants"}
	for _, executor := range []*liveToolExecutor{weatherTool, restaurantTool} {
		if err := sdk.RegisterTool(executor); err != nil {
			t.Fatalf("RegisterTool(%q) error = %v", executor.name, err)
		}
	}

	request := requestprotocol.NewRequest("deepseek-v4-pro")
	request.AddUserMessage("请分别调用 get_weather 和 search_restaurants 查询杭州天气和西湖附近餐厅，然后总结结果。不要编造工具结果。")
	request.AddFunctionTool(
		"get_weather",
		"查询指定城市的当前天气。",
		requestprotocol.NewObjectParameters(
			requestprotocol.String("location", "城市名称").Required(),
		),
	)
	request.AddFunctionTool(
		"search_restaurants",
		"搜索指定地点附近的餐厅。",
		requestprotocol.NewObjectParameters(
			requestprotocol.String("city", "城市名称").Required(),
			requestprotocol.String("near", "附近地点").Required(),
		),
	)
	maxTokens := 256
	request.MaxTokens = &maxTokens

	response, err := sdk.ChatWithTools(request)
	if err != nil {
		t.Fatalf("ChatWithTools() error = %v", err)
	}
	if weatherTool.calls == 0 || restaurantTool.calls == 0 {
		t.Fatalf("tool call counts: weather=%d restaurants=%d, want both tools called", weatherTool.calls, restaurantTool.calls)
	}
	if response.Message.Content == "" {
		t.Fatal("ChatWithTools() returned an empty final message")
	}
	if len(response.RoundUsages) < 2 {
		t.Fatalf("RoundUsages has %d rounds, want at least 2", len(response.RoundUsages))
	}
	if response.Usage == nil {
		t.Fatal("ChatWithTools() returned no aggregate usage")
	}
	finalJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		t.Fatalf("marshal final response: %v", err)
	}
	t.Logf("DeepSeek final raw response JSON: %s", response.Raw)
	t.Logf("DeepSeek normalized multi-round response JSON: %s", finalJSON)
}

func TestBailianThirdPartyModelsLiveToolLoop(t *testing.T) {
	apiKey := os.Getenv("QWEN_API_KEY")
	if apiKey == "" {
		t.Skip("QWEN_API_KEY is not set")
	}

	for _, model := range []string{"kimi-k2.6", "glm-5"} {
		t.Run(model, func(t *testing.T) {
			sdk, err := NewWithOptions("qwen", apiKey, linkprotocol.Options{
				Timeout: 90 * time.Second,
			})
			if err != nil {
				t.Fatalf("NewWithOptions() error = %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			if err := sdk.SetContext(ctx); err != nil {
				t.Fatalf("SetContext() error = %v", err)
			}

			weatherTool := &liveToolExecutor{name: "get_weather"}
			if err := sdk.RegisterTool(weatherTool); err != nil {
				t.Fatalf("RegisterTool() error = %v", err)
			}
			request := requestprotocol.NewRequest(model)
			request.AddUserMessage("为了准确回答，请调用 get_weather 查询杭州天气，再根据工具结果简短回答。不要编造结果。")
			request.AddFunctionTool(
				"get_weather",
				"查询指定城市的当前天气。",
				requestprotocol.NewObjectParameters(
					requestprotocol.String("location", "城市名称").Required(),
				),
			)
			maxTokens := 256
			request.MaxTokens = &maxTokens

			response, err := sdk.ChatWithTools(request)
			if err != nil {
				t.Fatalf("ChatWithTools() error = %v", err)
			}
			if weatherTool.calls == 0 {
				t.Fatal("model did not call get_weather")
			}
			if response.Message.Content == "" {
				t.Fatal("ChatWithTools() returned an empty final message")
			}
			if len(response.RoundUsages) < 2 {
				t.Fatalf("RoundUsages has %d rounds, want at least 2", len(response.RoundUsages))
			}
			finalJSON, err := json.MarshalIndent(response, "", "  ")
			if err != nil {
				t.Fatalf("marshal final response: %v", err)
			}
			t.Logf("%s final raw response JSON: %s", model, response.Raw)
			t.Logf("%s normalized multi-round response JSON: %s", model, finalJSON)
		})
	}
}
