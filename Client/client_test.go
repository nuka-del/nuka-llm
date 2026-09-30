package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	linkprotocol "github.com/nuka-del/nuka-llm/Link"
	alibaba "github.com/nuka-del/nuka-llm/Link/series/Alibaba"
	anthropic "github.com/nuka-del/nuka-llm/Link/series/Anthropic"
	deepseek "github.com/nuka-del/nuka-llm/Link/series/DeepSeek"
	moonshot "github.com/nuka-del/nuka-llm/Link/series/Moonshot"
	openai "github.com/nuka-del/nuka-llm/Link/series/OpenAi"
	zhipuai "github.com/nuka-del/nuka-llm/Link/series/ZhipuAI"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
	toolprotocol "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type testLink struct {
	rawCalls    int
	decodeCalls int
	lastContext context.Context
}

func (l *testLink) ChatRaw(ctx context.Context, _ requestprotocol.Request) ([]byte, error) {
	l.rawCalls++
	l.lastContext = ctx
	return []byte(`{"choices":[]}`), nil
}

func (*testLink) BuildRequest(requestprotocol.Request) ([]byte, error) {
	return nil, nil
}

func (*testLink) RegisterTool(toolprotocol.ToolExecutor) error { return nil }

func (*testLink) ChatWithToolResults(
	context.Context,
	*requestprotocol.Request,
	responseprotocol.Response,
) (responseprotocol.Response, error) {
	return responseprotocol.Response{}, nil
}

func (l *testLink) DecodeResponse(raw []byte) (responseprotocol.Response, error) {
	l.decodeCalls++
	return responseprotocol.Response{
		Provider: "test",
		Raw:      append([]byte(nil), raw...),
	}, nil
}

func TestChatUsesOneRawRequestThenDecodes(t *testing.T) {
	link := &testLink{}
	client := &Client{link: link}

	response, err := client.Chat(requestprotocol.Request{})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if response.Provider != "test" {
		t.Errorf("Provider = %q, want test", response.Provider)
	}
	if link.rawCalls != 1 {
		t.Errorf("ChatRaw calls = %d, want 1", link.rawCalls)
	}
	if link.decodeCalls != 1 {
		t.Errorf("DecodeResponse calls = %d, want 1", link.decodeCalls)
	}
}

func TestChatRawDoesNotDecode(t *testing.T) {
	link := &testLink{}
	client := &Client{link: link}

	raw, err := client.ChatRaw(requestprotocol.Request{})
	if err != nil {
		t.Fatalf("ChatRaw() error = %v", err)
	}
	if string(raw) != `{"choices":[]}` {
		t.Errorf("ChatRaw() = %s", raw)
	}
	if link.rawCalls != 1 || link.decodeCalls != 0 {
		t.Errorf("calls: raw=%d decode=%d, want raw=1 decode=0", link.rawCalls, link.decodeCalls)
	}
}

func TestNewWithOptionsRejectsBlankAPIKey(t *testing.T) {
	_, err := NewWithOptions("deepseek", "  ", linkprotocol.Options{})
	if err == nil {
		t.Fatal("NewWithOptions() error = nil, want API key validation error")
	}

	var sdkErr *sdkerror.SDKError
	if !errors.As(err, &sdkErr) {
		t.Fatalf("NewWithOptions() error type = %T, want *SDKError", err)
	}
	if sdkErr.Kind != sdkerror.InvalidRequest || sdkErr.Message != "API key is required" {
		t.Errorf("NewWithOptions() error = %#v", sdkErr)
	}
}

func TestClientContextFallsBackAndCanBeConfigured(t *testing.T) {
	client := &Client{link: &testLink{}}
	if err := client.SetContext(nil); err != nil {
		t.Fatalf("SetContext(nil) error = %v", err)
	}
	if _, err := client.ChatRaw(requestprotocol.Request{}); err != nil {
		t.Fatalf("ChatRaw() with default context error = %v", err)
	}
	link := client.link.(*testLink)
	if link.lastContext == nil {
		t.Fatal("ChatRaw() received nil fallback context")
	}

	configuredContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := client.SetContext(configuredContext); err != nil {
		t.Fatalf("SetContext() error = %v", err)
	}
	if _, err := client.ChatRaw(requestprotocol.Request{}); err != nil {
		t.Fatalf("ChatRaw() with configured context error = %v", err)
	}
	if link.lastContext != configuredContext {
		t.Fatal("ChatRaw() did not use the configured Client context")
	}

	var zeroValue Client
	if _, err := zeroValue.Chat(requestprotocol.Request{}); err == nil {
		t.Fatal("zero-value Client.Chat() error = nil, want validation error")
	} else {
		var sdkErr *sdkerror.SDKError
		if !errors.As(err, &sdkErr) || sdkErr.Message != "client is not initialized" {
			t.Errorf("zero-value Client.Chat() error = %v", err)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type staticToolExecutor struct {
	name   string
	result string
	calls  int
}

func (t *staticToolExecutor) Name() string {
	return t.name
}

func (t *staticToolExecutor) Start(arguments json.RawMessage) (toolprotocol.ToolResult, error) {
	t.calls++
	if !json.Valid(arguments) {
		return toolprotocol.ToolResult{}, errors.New("tool arguments are not valid JSON")
	}
	return toolprotocol.ToolResult{Content: t.result}, nil
}

func TestChatWithToolsRunsMultipleRoundsAndAggregatesUsage(t *testing.T) {
	responses := []string{
		`{"id":"round-1","model":"deepseek-v4-pro","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"call-weather","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"杭州\"}"}},{"id":"call-restaurants","type":"function","function":{"name":"search_restaurants","arguments":"{\"city\":\"杭州\",\"near\":\"西湖\"}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`,
		`{"id":"round-2","model":"deepseek-v4-pro","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"call-air","type":"function","function":{"name":"get_air_quality","arguments":"{\"location\":\"杭州\"}"}}]}}],"usage":{"prompt_tokens":20,"completion_tokens":5,"total_tokens":25}}`,
		`{"id":"round-3","model":"deepseek-v4-pro","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"天气晴朗，西湖附近有两家餐厅，空气质量良好。"}}],"usage":{"prompt_tokens":30,"completion_tokens":6,"total_tokens":36}}`,
	}
	var requestBodies [][]byte
	httpClient := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
			requestBodies = append(requestBodies, body)
			if len(requestBodies) > len(responses) {
				return nil, errors.New("unexpected extra HTTP request")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(responses[len(requestBodies)-1])),
				Request:    request,
			}, nil
		}),
	}

	sdk, err := NewWithOptions("deepseek", "test-key", linkprotocol.Options{
		HTTPClient: httpClient,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	weatherTool := &staticToolExecutor{
		name:   "get_weather",
		result: `{"weather":"晴","temperature_c":26}`,
	}
	restaurantTool := &staticToolExecutor{
		name:   "search_restaurants",
		result: `{"restaurants":["湖畔餐厅","山外山"]}`,
	}
	airTool := &staticToolExecutor{
		name:   "get_air_quality",
		result: `{"aqi":32}`,
	}
	for _, executor := range []*staticToolExecutor{weatherTool, restaurantTool, airTool} {
		if err := sdk.RegisterTool(executor); err != nil {
			t.Fatalf("RegisterTool(%q) error = %v", executor.name, err)
		}
	}

	request := requestprotocol.NewRequest("deepseek-v4-pro")
	request.AddUserMessage("查询杭州天气、西湖附近餐厅和空气质量。")
	request.AddFunctionTool("get_weather", "查询城市天气。", requestprotocol.NewObjectParameters(
		requestprotocol.String("location", "城市名称").Required(),
	))
	request.AddFunctionTool("search_restaurants", "搜索附近餐厅。", requestprotocol.NewObjectParameters(
		requestprotocol.String("city", "城市名称").Required(),
		requestprotocol.String("near", "附近地点").Required(),
	))
	request.AddFunctionTool("get_air_quality", "查询空气质量。", requestprotocol.NewObjectParameters(
		requestprotocol.String("location", "城市名称").Required(),
	))

	response, err := sdk.ChatWithTools(request)
	if err != nil {
		t.Fatalf("ChatWithTools() error = %v", err)
	}
	if len(requestBodies) != 3 {
		t.Fatalf("HTTP request count = %d, want 3", len(requestBodies))
	}
	if weatherTool.calls != 1 || restaurantTool.calls != 1 || airTool.calls != 1 {
		t.Errorf("tool calls = weather:%d restaurants:%d air:%d, want 1 each", weatherTool.calls, restaurantTool.calls, airTool.calls)
	}
	if response.Message.Content != "天气晴朗，西湖附近有两家餐厅，空气质量良好。" {
		t.Errorf("final Message.Content = %q", response.Message.Content)
	}
	if response.Usage == nil || response.Usage.InputTokens != 60 || response.Usage.OutputTokens != 15 || response.Usage.TotalTokens != 75 {
		t.Errorf("aggregated Usage = %#v", response.Usage)
	}
	if len(response.RoundUsages) != 3 {
		t.Fatalf("RoundUsages length = %d, want 3", len(response.RoundUsages))
	}
	for i, roundUsage := range response.RoundUsages {
		if roundUsage.Round != i+1 {
			t.Errorf("RoundUsages[%d].Round = %d, want %d", i, roundUsage.Round, i+1)
		}
	}

	var secondRequest struct {
		Messages []struct {
			Role       string `json:"role"`
			ToolCallID string `json:"tool_call_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(requestBodies[1], &secondRequest); err != nil {
		t.Fatalf("decode second request: %v", err)
	}
	if len(secondRequest.Messages) != 4 {
		t.Fatalf("second request message count = %d, want 4", len(secondRequest.Messages))
	}
	if secondRequest.Messages[2].Role != "tool" || secondRequest.Messages[2].ToolCallID != "call-weather" {
		t.Errorf("first tool result message = %#v", secondRequest.Messages[2])
	}
	if secondRequest.Messages[3].Role != "tool" || secondRequest.Messages[3].ToolCallID != "call-restaurants" {
		t.Errorf("second tool result message = %#v", secondRequest.Messages[3])
	}

	var thirdRequest struct {
		Messages []struct {
			Role       string `json:"role"`
			ToolCallID string `json:"tool_call_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(requestBodies[2], &thirdRequest); err != nil {
		t.Fatalf("decode third request: %v", err)
	}
	if len(thirdRequest.Messages) != 6 {
		t.Fatalf("third request message count = %d, want 6", len(thirdRequest.Messages))
	}
	if thirdRequest.Messages[5].Role != "tool" || thirdRequest.Messages[5].ToolCallID != "call-air" {
		t.Errorf("third request final tool result = %#v", thirdRequest.Messages[5])
	}
}

func TestKimiChatWithToolsPreservesReasoningContent(t *testing.T) {
	responses := []string{
		`{"id":"kimi-round-1","model":"kimi-k3","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":"","reasoning_content":"call the weather tool","tool_calls":[{"id":"call-weather","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"杭州\"}"}}]}}],"usage":{"prompt_tokens":8,"completion_tokens":4,"total_tokens":12}}`,
		`{"id":"kimi-round-2","model":"kimi-k3","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"杭州天气晴。"}}],"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16}}`,
	}
	var requestBodies [][]byte
	httpClient := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
			requestBodies = append(requestBodies, body)
			if len(requestBodies) > len(responses) {
				return nil, errors.New("unexpected extra HTTP request")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(responses[len(requestBodies)-1])),
				Request:    request,
			}, nil
		}),
	}

	sdk, err := NewWithOptions("kimi", "test-key", linkprotocol.Options{
		HTTPClient: httpClient,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if err := sdk.RegisterTool(&staticToolExecutor{
		name:   "get_weather",
		result: `{"weather":"晴"}`,
	}); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}

	request := requestprotocol.NewRequest("kimi-k3")
	request.AddUserMessage("查询杭州天气。")
	request.AddFunctionTool(
		"get_weather",
		"查询指定城市的当前天气。",
		requestprotocol.NewObjectParameters(
			requestprotocol.String("location", "城市名称").Required(),
		),
	)
	response, err := sdk.ChatWithTools(request)
	if err != nil {
		t.Fatalf("ChatWithTools() error = %v", err)
	}
	if response.Message.Content != "杭州天气晴。" {
		t.Errorf("final Message.Content = %q", response.Message.Content)
	}
	if len(requestBodies) != 2 {
		t.Fatalf("HTTP request count = %d, want 2", len(requestBodies))
	}

	var followUp struct {
		Messages []map[string]json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(requestBodies[1], &followUp); err != nil {
		t.Fatalf("decode follow-up request: %v", err)
	}
	var reasoning string
	if err := json.Unmarshal(followUp.Messages[1]["reasoning_content"], &reasoning); err != nil {
		t.Fatalf("decode reasoning_content: %v", err)
	}
	if reasoning != "call the weather tool" {
		t.Errorf("follow-up reasoning_content = %q", reasoning)
	}
}

func TestReasoningContentIsOnlySerializedForKimi(t *testing.T) {
	maxTokens := 64
	request := requestprotocol.Request{
		Model: "test-model",
		Messages: []requestprotocol.Message{
			{
				Role:             "assistant",
				Content:          "answer",
				ReasoningContent: "private reasoning",
			},
			{
				Role:             "user",
				Content:          "question",
				ReasoningContent: "not an assistant field",
			},
		},
		MaxTokens: &maxTokens,
	}
	options := linkprotocol.DefaultOptions()
	compatibleProviders := []struct {
		name string
		link linkprotocol.Link
	}{
		{name: "openai", link: openai.NewWithOptions("test-key", options)},
		{name: "deepseek", link: deepseek.NewWithOptions("test-key", options)},
		{name: "qwen", link: alibaba.NewWithOptions("test-key", options)},
		{name: "zhipuai", link: zhipuai.NewWithOptions("test-key", options)},
		{name: "anthropic", link: anthropic.NewWithOptions("test-key", options)},
	}
	for _, provider := range compatibleProviders {
		t.Run(provider.name, func(t *testing.T) {
			body, err := provider.link.BuildRequest(request)
			if err != nil {
				t.Fatalf("BuildRequest() error = %v", err)
			}
			if strings.Contains(string(body), "reasoning_content") {
				t.Errorf("BuildRequest() unexpectedly contains reasoning_content: %s", body)
			}
		})
	}

	kimi := moonshot.NewWithOptions("test-key", options)
	body, err := kimi.BuildRequest(request)
	if err != nil {
		t.Fatalf("Kimi BuildRequest() error = %v", err)
	}
	var payload struct {
		Messages []map[string]json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode Kimi request: %v", err)
	}
	var reasoning string
	if err := json.Unmarshal(payload.Messages[0]["reasoning_content"], &reasoning); err != nil {
		t.Fatalf("decode assistant reasoning_content: %v", err)
	}
	if reasoning != "private reasoning" {
		t.Errorf("assistant reasoning_content = %q", reasoning)
	}
	if _, exists := payload.Messages[1]["reasoning_content"]; exists {
		t.Error("Kimi request includes reasoning_content on a user message")
	}
}

func TestNewWithOptionsUsesProvidedHTTPClient(t *testing.T) {
	const responseBody = `{"id":"test-id","model":"deepseek-v4-pro","choices":[{"index":0,"message":{"role":"assistant","content":"你好"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`
	requestReceived := false
	httpClient := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestReceived = true
			if request.URL.String() != "https://api.deepseek.com/chat/completions" {
				t.Errorf("request URL = %q", request.URL)
			}
			if request.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("Authorization header = %q", request.Header.Get("Authorization"))
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("read request body: %v", err)
			}
			if !strings.Contains(string(body), `"model":"deepseek-v4-pro"`) {
				t.Errorf("request body missing model: %s", body)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(responseBody)),
				Request:    request,
			}, nil
		}),
	}

	sdk, err := NewWithOptions("deepseek", "test-key", linkprotocol.Options{
		HTTPClient: httpClient,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	response, err := sdk.Chat(requestprotocol.Request{
		Model:    "deepseek-v4-pro",
		Messages: []requestprotocol.Message{{Role: "user", Content: "你好"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if !requestReceived {
		t.Fatal("configured HTTP client did not receive the request")
	}
	if len(response.Choices) != 1 || response.Choices[0].Message.Content != "你好" {
		t.Fatalf("Chat() response = %#v", response)
	}
}

func TestNewWithOptionsRoutesOpenAICompatibleProviders(t *testing.T) {
	providers := []struct {
		series   string
		endpoint string
		provider string
	}{
		{
			series:   "deepseek",
			endpoint: "https://api.deepseek.com/chat/completions",
			provider: "deepseek",
		},
		{
			series:   "qwen",
			endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
			provider: "qwen",
		},
		{
			series:   "glm",
			endpoint: "https://open.bigmodel.cn/api/paas/v4/chat/completions",
			provider: "glm",
		},
		{
			series:   "kimi",
			endpoint: "https://api.moonshot.cn/v1/chat/completions",
			provider: "moonshot",
		},
	}

	for _, provider := range providers {
		t.Run(provider.series, func(t *testing.T) {
			const responseBody = `{"id":"test-id","model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`
			requestReceived := false
			httpClient := &http.Client{
				Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					requestReceived = true
					if request.URL.String() != provider.endpoint {
						t.Errorf("request URL = %q, want %q", request.URL, provider.endpoint)
					}
					if request.Header.Get("Authorization") != "Bearer test-key" {
						t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(responseBody)),
						Request:    request,
					}, nil
				}),
			}

			sdk, err := NewWithOptions(provider.series, "test-key", linkprotocol.Options{
				HTTPClient: httpClient,
				Timeout:    time.Second,
			})
			if err != nil {
				t.Fatalf("NewWithOptions() error = %v", err)
			}
			request := requestprotocol.NewRequest("test-model")
			request.AddUserMessage("hello")
			response, err := sdk.Chat(request)
			if err != nil {
				t.Fatalf("Chat() error = %v", err)
			}
			if !requestReceived {
				t.Fatal("configured HTTP client did not receive the request")
			}
			if response.Provider != provider.provider || response.Message.Content != "OK" {
				t.Errorf("Chat() response = %#v", response)
			}
		})
	}
}

func TestOptionsTimeoutIsUsedWithoutCallerDeadline(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	}
	sdk, err := NewWithOptions("deepseek", "test-key", linkprotocol.Options{
		HTTPClient: httpClient,
		Timeout:    20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	_, err = sdk.ChatRaw(requestprotocol.Request{
		Model:    "deepseek-v4-pro",
		Messages: []requestprotocol.Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("ChatRaw() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("ChatRaw() error = %v, want context deadline exceeded", err)
	}
	var sdkErr *sdkerror.SDKError
	if !errors.As(err, &sdkErr) || sdkErr.Kind != sdkerror.Transport {
		t.Errorf("ChatRaw() error = %T (%v), want Transport SDKError", err, err)
	}
}
