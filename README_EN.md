# nuka-llm

[English](README_EN.md) | [简体中文](README.md) | [日本語](README_JA.md)

`nuka-llm` is a Go multi-provider LLM Agent SDK. Regular chat is the basic model interaction; its core workflow dispatches model-generated tool calls to registered application tools, returns their results to the model, and repeats until a final answer is produced.

## Features

- Multiple providers: DeepSeek, OpenAI, Alibaba Cloud Bailian Qwen, Zhipu GLM, Moonshot Kimi, and Anthropic Claude.
- A tool registry that routes model-generated `tool_calls` to application-provided executors.
- Automatic tool execution, tool-call ID correlation, result handoff, and multi-round orchestration.
- Per-round token usage and aggregated usage across tool-calling rounds.
- A unified synchronous chat API, response type, and builder methods for requests, messages, and function tools.
- Configurable HTTP client, request timeout, and Client context.

## Core workflow

```text
User prompt + tool definitions
            ↓
Client.ChatWithTools → provider builds its request format
            ↓
Model returns tool_calls
            ↓
ToolRegistry finds each executor by function name and passes Arguments
            ↓
Execute tools and collect ToolResult values
            ↓
Provider appends assistant/tool messages and calls the model again
            ↓
Repeat until the model returns a final answer
```

Tools represent capabilities supplied by the application, such as querying weather, accessing a database, calling internal services, or running workflows. The SDK orchestrates the model/tool loop; the application implements the actual tool logic.

## Installation

Requires Go 1.26.4 or later.

```bash
go get github.com/nuka-del/nuka-llm
```

## Provider selectors

Pass one of the following provider selectors to `client.New` or `client.NewWithOptions`. Specify the model name in `Request.Model`.

| Provider selector | Provider | Default Chat Completions endpoint | Example models |
| --- | --- | --- | --- |
| `deepseek` | DeepSeek | `https://api.deepseek.com/chat/completions` | `deepseek-v4-pro` |
| `chatgpt` | OpenAI | `https://api.openai.com/v1/chat/completions` | Models supported by OpenAI |
| `qwen` | Alibaba Cloud Bailian | `https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions` | `qwen-plus`, `kimi-k2.6`, `glm-5` |
| `glm` | Zhipu AI | `https://open.bigmodel.cn/api/paas/v4/chat/completions` | `glm-5.3` |
| `kimi` | Moonshot AI | `https://api.moonshot.cn/v1/chat/completions` | `kimi-k3` |
| `cluade` | Anthropic Claude | `https://api.anthropic.com/v1/messages` | Models supported by Anthropic |

Use an Alibaba Cloud Bailian API key with the `qwen` selector for Bailian-hosted `kimi-k2.6` or `glm-5`. The `kimi` and `glm` selectors connect to the Moonshot AI and Zhipu AI endpoints and require API keys for those services. The Anthropic selector is spelled `cluade` in the current Client API.
Anthropic requests must set `Request.MaxTokens`.

## Basic capability: regular chat

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
	request.AddSystem("You are a helpful assistant.")
	request.AddUserMessage("Explain Go in one sentence.")

	response, err := sdk.Chat(request)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response.Message.Content)
}
```

## Client context and HTTP options

Set the context on the Client once. `Chat`, `ChatRaw`, and `ChatWithTools` all use it. If no context is set, or `SetContext(nil)` is called, the Client falls back to `context.Background()`.

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

Use `NewWithOptions` to provide a custom HTTP client or timeout:

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

If `HTTPClient` is nil, `http.DefaultClient` is used. If `Timeout` is unset or non-positive, the default request timeout is five seconds. Providers respect a deadline already present on the Client context.

## Building requests

Append common messages without manually assembling nested `Message` values:

```go
request := requestprotocol.NewRequest("deepseek-v4-pro")
request.AddSystem("You are a helpful assistant.")
request.AddUserMessage("Check the weather in Hangzhou.")
```

Initial messages can also be passed to `NewRequest`:

```go
request := requestprotocol.NewRequest(
	"deepseek-v4-pro",
	requestprotocol.Message{Role: "user", Content: "Hello."},
)
```

Simple function-tool object properties can be built with `String`, `Number`, `Integer`, and `Boolean`. Mark required fields with `.Required()`:

```go
request.AddFunctionTool(
	"get_weather",
	"Get current weather for a city.",
	requestprotocol.NewObjectParameters(
		requestprotocol.String("location", "City name").Required(),
		requestprotocol.String("unit", "Temperature unit"),
	),
)
```

Tool definitions are appended to `request.Tools`. `AddAssistantMessage`, `AddToolResult`, and the generic `AddMessage` method are available for manually managing conversation history.

Advanced request settings remain directly configurable:

```go
maxTokens := 256
request.MaxTokens = &maxTokens

temperature := 0.2
request.Temperature = &temperature
```

## Registering and implementing tools

Implement `toolprotocol.ToolExecutor`. `Start` receives the model's JSON arguments and returns a non-empty `ToolResult.Content`. The SDK associates the result with the original `ToolCallID`.

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
		return toolprotocol.ToolResult{}, fmt.Errorf("parse weather arguments: %w", err)
	}

	data, err := json.Marshal(map[string]any{
		"location":      input.Location,
		"weather":       "sunny",
		"temperature_c": 26,
	})
	if err != nil {
		return toolprotocol.ToolResult{}, err
	}
	return toolprotocol.ToolResult{Content: string(data)}, nil
}
```

Register the executor and check the returned error:

```go
if err := sdk.RegisterTool(WeatherTool{}); err != nil {
	return err
}
```

## Automatic multi-round tool calling

Declare tool schemas in the Request and register matching executors on the Client. `ChatWithTools` sends the initial request, executes requested tools, appends their results using the corresponding tool-call IDs, and continues until the model returns a regular answer.

```go
request := requestprotocol.NewRequest("deepseek-v4-pro")
request.AddSystem("You are a helpful assistant.")
request.AddUserMessage("Check the weather in Hangzhou and summarize it.")
request.AddFunctionTool(
	"get_weather",
	"Get current weather for a city.",
	requestprotocol.NewObjectParameters(
		requestprotocol.String("location", "City name").Required(),
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

Register additional executors and call `AddFunctionTool` for each tool. The tool schema name must match the executor's `Name()`. Tool execution errors, unregistered tools, missing tool-call IDs, and empty results are returned as SDK errors.

## Response and token usage

Both `Chat` and `ChatWithTools` return `responseprotocol.Response`. `Message` is a convenience field for the first choice; all choices remain available in `Choices`.

```go
fmt.Println(response.Message.Content)
fmt.Println(response.Message.ToolCalls)

if response.Usage != nil {
	fmt.Println("Input tokens:", response.Usage.InputTokens)
	fmt.Println("Output tokens:", response.Usage.OutputTokens)
	fmt.Println("Total tokens:", response.Usage.TotalTokens)
}

for _, round := range response.RoundUsages {
	if round.Usage != nil {
		fmt.Printf("Round %d: %+v\n", round.Round, *round.Usage)
	}
}
```

For a regular `Chat`, `RoundUsages` contains one entry. For `ChatWithTools`, `Usage` is aggregated across the model requests and `RoundUsages` records usage for each request. `Raw` contains the provider's raw response for the final round; it is excluded when the normalized `Response` is JSON-encoded.

## Using a provider directly

You can use a provider without `Client`. In that case, pass a `context.Context` to provider methods:

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
	request.AddUserMessage("Check the weather in Hangzhou.")
	request.AddFunctionTool(
		"get_weather",
		"Get current weather for a city.",
		requestprotocol.NewObjectParameters(
			requestprotocol.String("location", "City name").Required(),
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

`ChatWithToolResults` executes one batch of tool calls and sends the follow-up request. `Client.ChatWithTools` handles the automatic multi-round loop.

## Error handling

Errors are returned as `*sdkerror.SDKError`, with provider, error kind, HTTP status, message, and an optional cause. Use `errors.As` to inspect them:

```go
import (
	"errors"
	"fmt"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
)

var sdkErr *sdkerror.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr.Provider, sdkErr.Kind, sdkErr.Message)
}
```

Error kinds include `invalid_request`, `transport`, `api`, `decode`, and `tool_execution`.

## Running tests

Run unit and simulated workflow tests:

```bash
go test ./...
```

Optional live API tests read environment variables and skip when a key is unset:

| Environment variable | Tests |
| --- | --- |
| `DEEPSEEK_API_KEY` | DeepSeek chat and a real tool-calling loop |
| `QWEN_API_KEY` | Bailian `qwen-plus`, `kimi-k2.6`, and `glm-5` chat and tool calls |
| `GLM_API_KEY` | Direct Zhipu AI GLM chat |
| `KIMI_API_KEY` or `MOONSHOT_API_KEY` | Direct Moonshot Kimi chat |

PowerShell example:

```powershell
$env:DEEPSEEK_API_KEY = "your DeepSeek API key"
$env:QWEN_API_KEY = "your Alibaba Cloud Bailian API key"
go test ./... -count=1
```

Keep API keys in environment variables or local configuration; never commit them.
