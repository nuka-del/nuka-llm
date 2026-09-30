# nuka-llm

`nuka-llm` 是一个 Go 多模型聊天 SDK。它为多个模型服务商提供统一的请求、响应和工具调用接口，同时保留各 provider 自己的 HTTP endpoint 与 JSON 映射。

## 功能

- 统一的同步聊天接口和响应结构。
- 多 provider 支持：DeepSeek、OpenAI、阿里云百炼 Qwen、智谱 GLM、Moonshot Kimi 和 Anthropic Claude。
- 普通聊天和自动多轮工具调用。
- 工具注册、参数传递、工具结果关联及执行错误处理。
- 逐轮 token 用量和多轮累计用量。
- 简化的 Request、Message 和 Function Tool 构造方法。
- 可配置 HTTP client、请求超时和 Client context。

## 安装

需要 Go 1.26.4 或更新版本。

```bash
go get github.com/nuka-del/nuka-llm
```

## Provider 名称

`client.New` 和 `client.NewWithOptions` 使用下表中的 provider 名称。模型名称放在 Request 的 `Model` 字段中。

| Provider 名称 | 服务商 | 默认 Chat Completions endpoint | 模型示例 |
| --- | --- | --- | --- |
| `deepseek` | DeepSeek | `https://api.deepseek.com/chat/completions` | `deepseek-v4-pro` |
| `chatgpt` | OpenAI | `https://api.openai.com/v1/chat/completions` | 由 OpenAI 支持的模型名称 |
| `qwen` | 阿里云百炼 | `https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions` | `qwen-plus`、`kimi-k2.6`、`glm-5` |
| `glm` | 智谱 AI | `https://open.bigmodel.cn/api/paas/v4/chat/completions` | `glm-5.3` |
| `kimi` | Moonshot AI | `https://api.moonshot.cn/v1/chat/completions` | `kimi-k3` |
| `cluade` | Anthropic Claude | `https://api.anthropic.com/v1/messages` | 由 Anthropic 支持的模型名称 |

通过 `qwen` 使用 `kimi-k2.6` 或 `glm-5` 时，请使用阿里云百炼的 API key。`kimi` 和 `glm` 则连接 Moonshot AI 与智谱 AI 各自的 endpoint，需要对应服务商的 API key。
Anthropic 请求需要设置 `Request.MaxTokens`。

## 快速开始：普通聊天

```go
package main

import (
	"fmt"
	"log"
	"os"

	client "github.com/nuka-del/nuka-llm/Client"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
)

func main() {
	sdk, err := client.New("deepseek", os.Getenv("DEEPSEEK_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	request := requestprotocol.NewRequest("deepseek-v4-pro")
	request.AddSystem("你是一个有帮助的助手。")
	request.AddUserMessage("请用一句话介绍 Go 语言。")

	response, err := sdk.Chat(request)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response.Message.Content)
}
```

## Client context 和 HTTP 配置

Client context 在 Client 上设置一次，`Chat`、`ChatRaw` 和 `ChatWithTools` 共用它。没有设置 context，或调用 `SetContext(nil)` 时，Client 使用 `context.Background()`。

```go
import (
	"context"
	"time"

	client "github.com/nuka-del/nuka-llm/Client"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
)

func chatWithDeadline(sdk *client.Client, request requestprotocol.Request) (responseprotocol.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := sdk.SetContext(ctx); err != nil {
		return responseprotocol.Response{}, err
	}
	return sdk.Chat(request)
}
```

需要自定义 HTTP client 或超时时，使用 `NewWithOptions`：

```go
import (
	"net/http"
	"time"

	client "github.com/nuka-del/nuka-llm/Client"
	link "github.com/nuka-del/nuka-llm/Link"
)

func newClient(apiKey string) (*client.Client, error) {
	return client.NewWithOptions(
		"deepseek",
		apiKey,
		link.Options{
			HTTPClient: &http.Client{},
			Timeout:    30 * time.Second,
		},
	)
}
```

`HTTPClient` 未设置时使用 `http.DefaultClient`；`Timeout` 未设置或小于等于零时使用默认的 5 秒请求超时。调用 context 自带 deadline 时，provider 会遵循该 deadline。

## Request 构造方法

常用消息可以通过方法追加，不需要手工构造多层 `Message` 结构：

```go
request := requestprotocol.NewRequest("deepseek-v4-pro")
request.AddSystem("你是一个有帮助的助手。")
request.AddUserMessage("查询杭州天气。")
```

也可以通过 `NewRequest` 一次传入初始消息：

```go
request := requestprotocol.NewRequest(
	"deepseek-v4-pro",
	requestprotocol.Message{Role: "user", Content: "你好。"},
)
```

目前提供的简单 Function Tool 参数构造器包括 `String`、`Number`、`Integer` 和 `Boolean`。属性可通过 `.Required()` 标为必填：

```go
request.AddFunctionTool(
	"get_weather",
	"查询指定城市的当前天气。",
	requestprotocol.NewObjectParameters(
		requestprotocol.String("location", "城市名称").Required(),
		requestprotocol.String("unit", "温度单位"),
	),
)
```

工具定义会被加入 `request.Tools`。`AddAssistantMessage`、`AddToolResult` 和通用的 `AddMessage` 也可用于手动管理消息历史。
Kimi 返回的 assistant `reasoning_content` 会保存在统一响应中，并在 Kimi 的后续工具请求中保留；其他 provider 的 `BuildRequest` 不会序列化这个字段。

`Request` 的进阶字段仍可直接设置，例如：

```go
maxTokens := 256
request.MaxTokens = &maxTokens

temperature := 0.2
request.Temperature = &temperature
```

## 注册和实现工具

具体工具实现 `toolprotocol.ToolExecutor`。`Start` 接收模型返回的 JSON 参数，返回非空的 `ToolResult.Content`；对应的 `ToolCallID` 由 SDK 从模型的 tool call 中关联。

```go
package main

import (
	"encoding/json"
	"fmt"

	toolprotocol "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type WeatherTool struct{}

func (WeatherTool) Name() string {
	return "get_weather"
}

func (WeatherTool) Start(arguments json.RawMessage) (toolprotocol.ToolResult, error) {
	var input struct {
		Location string `json:"location"`
	}
	if err := json.Unmarshal(arguments, &input); err != nil {
		return toolprotocol.ToolResult{}, fmt.Errorf("解析天气参数: %w", err)
	}

	data, err := json.Marshal(map[string]any{
		"location":      input.Location,
		"weather":       "晴",
		"temperature_c": 26,
	})
	if err != nil {
		return toolprotocol.ToolResult{}, err
	}
	return toolprotocol.ToolResult{Content: string(data)}, nil
}
```

注册工具时检查错误：

```go
if err := sdk.RegisterTool(WeatherTool{}); err != nil {
	return err
}
```

## 自动多轮工具调用

在 Request 中声明工具 schema，并向 Client 注册同名的 executor。`ChatWithTools` 会发送首轮请求、运行模型请求的工具、把结果关联到 tool call ID，再继续请求，直到模型返回普通回答。

```go
request := requestprotocol.NewRequest("deepseek-v4-pro")
request.AddSystem("你是一个有帮助的助手。")
request.AddUserMessage("查询杭州天气，并简短总结。")
request.AddFunctionTool(
	"get_weather",
	"查询指定城市的当前天气。",
	requestprotocol.NewObjectParameters(
		requestprotocol.String("location", "城市名称").Required(),
	),
)

if err := sdk.RegisterTool(WeatherTool{}); err != nil {
	return err
}

response, err := sdk.ChatWithTools(request)
if err != nil {
	return err
}
fmt.Println(response.Message.Content)
```

多个工具可重复调用 `RegisterTool` 和 `AddFunctionTool`。工具名应与实现的 `Name()` 相同。工具执行失败、工具未注册、tool call ID 缺失或结果内容为空时，SDK 返回错误。

## Response 和 token 用量

`Chat` 与 `ChatWithTools` 都返回 `responseprotocol.Response`。`Message` 是首个 choice 的便捷访问字段；完整 choices 仍保存在 `Choices` 中。

```go
fmt.Println(response.Message.Content)
fmt.Println(response.Message.ToolCalls)

if response.Usage != nil {
	fmt.Println("总输入 token:", response.Usage.InputTokens)
	fmt.Println("总输出 token:", response.Usage.OutputTokens)
	fmt.Println("总 token:", response.Usage.TotalTokens)
}

for _, round := range response.RoundUsages {
	if round.Usage != nil {
		fmt.Printf("第 %d 轮: %+v\n", round.Round, *round.Usage)
	}
}
```

普通 `Chat` 的 `RoundUsages` 记录一次请求。`ChatWithTools` 的 `Usage` 是整个工具对话的累计用量，`RoundUsages` 记录每一次模型请求的用量。`Raw` 保留最后一轮 provider 原始响应，可直接读取，但不会随 `Response` 的 JSON 序列化输出。

## 直接使用 provider

如果不使用 `Client`，可以直接创建具体 provider。直接使用时由调用方传入 `context.Context`：

```go
import (
	"context"
	"fmt"
	"os"
	"time"

	deepseek "github.com/nuka-del/nuka-llm/Link/series/DeepSeek"
	link "github.com/nuka-del/nuka-llm/Link"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
)

func chatDirectly() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	provider := deepseek.NewWithOptions(os.Getenv("DEEPSEEK_API_KEY"), link.DefaultOptions())
	if err := provider.RegisterTool(WeatherTool{}); err != nil {
		return err
	}

	request := requestprotocol.NewRequest("deepseek-v4-pro")
	request.AddUserMessage("查询杭州天气。")
	request.AddFunctionTool(
		"get_weather",
		"查询指定城市的当前天气。",
		requestprotocol.NewObjectParameters(
			requestprotocol.String("location", "城市名称").Required(),
		),
	)
	raw, err := provider.ChatRaw(ctx, request)
	if err != nil {
		return err
	}
	firstResponse, err := provider.DecodeResponse(raw)
	if err != nil {
		return err
	}
	if len(firstResponse.Message.ToolCalls) == 0 {
		fmt.Println(firstResponse.Message.Content)
		return nil
	}
	finalResponse, err := provider.ChatWithToolResults(ctx, &request, firstResponse)
	if err != nil {
		return err
	}
	fmt.Println(finalResponse.Message.Content)
	return nil
}
```

`ChatWithToolResults` 处理一次工具调用批次并发送后续请求；Client 的 `ChatWithTools` 负责自动循环多轮。

## 错误处理

SDK 错误类型是 `*sdkerror.SDKError`，包含 provider、错误类别、HTTP 状态码、消息和底层原因。可用 `errors.As` 判断：

```go
var sdkErr *sdkerror.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr.Provider, sdkErr.Kind, sdkErr.Message)
}
```

当前错误类别包括：`invalid_request`、`transport`、`api`、`decode` 和 `tool_execution`。

## 运行测试

运行单元及模拟流程测试：

```bash
go test ./...
```

可选真实 API 测试读取环境变量；未设置对应 key 时会跳过：

| 环境变量 | 用途 |
| --- | --- |
| `DEEPSEEK_API_KEY` | DeepSeek 聊天和真实工具循环 |
| `QWEN_API_KEY` | 百炼 `qwen-plus`、`kimi-k2.6`、`glm-5` 聊天和工具调用 |
| `GLM_API_KEY` | 智谱 AI 直连 GLM live test |
| `KIMI_API_KEY` 或 `MOONSHOT_API_KEY` | Moonshot 直连 Kimi live test |

PowerShell 示例：

```powershell
$env:DEEPSEEK_API_KEY = "你的 DeepSeek API key"
$env:QWEN_API_KEY = "你的阿里云百炼 API key"
go test ./... -count=1
```

API key 请通过环境变量或本地配置管理，不要提交到仓库。
