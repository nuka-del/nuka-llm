# nuka-llm

[日本語](README_JA.md) | [简体中文](README.md) | [English](README_EN.md)

`nuka-llm` は、複数のモデルプロバイダーを統一されたリクエスト、レスポンス、ツール呼び出し API で利用するための Go SDK です。各プロバイダー固有の HTTP endpoint と JSON マッピングも維持します。

## 機能

- 同期チャット API と統一レスポンス構造。
- DeepSeek、OpenAI、Alibaba Cloud Bailian Qwen、Zhipu GLM、Moonshot Kimi、Anthropic Claude に対応。
- 通常のチャットと自動マルチターン・ツール呼び出し。
- ツール登録、引数の受け渡し、tool call ID の対応付け、実行エラー処理。
- ターンごとの token 使用量と、ツール会話全体の累計使用量。
- Request、Message、Function Tool の簡易コンストラクター。
- HTTP client、リクエスト timeout、Client context の設定。

## インストール

Go 1.26.4 以降が必要です。

```bash
go get github.com/nuka-del/nuka-llm
```

## Provider selector

`client.New` または `client.NewWithOptions` に、以下の provider selector を指定します。モデル名は Request の `Model` に指定してください。

| Provider selector | プロバイダー | デフォルト Chat Completions endpoint | モデル例 |
| --- | --- | --- | --- |
| `deepseek` | DeepSeek | `https://api.deepseek.com/chat/completions` | `deepseek-v4-pro` |
| `chatgpt` | OpenAI | `https://api.openai.com/v1/chat/completions` | OpenAI がサポートするモデル名 |
| `qwen` | Alibaba Cloud Bailian | `https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions` | `qwen-plus`、`kimi-k2.6`、`glm-5` |
| `glm` | Zhipu AI | `https://open.bigmodel.cn/api/paas/v4/chat/completions` | `glm-5.3` |
| `kimi` | Moonshot AI | `https://api.moonshot.cn/v1/chat/completions` | `kimi-k3` |
| `cluade` | Anthropic Claude | `https://api.anthropic.com/v1/messages` | Anthropic がサポートするモデル名 |

Bailian が提供する `kimi-k2.6` または `glm-5` を使う場合は、Alibaba Cloud Bailian の API key と `qwen` selector を使用してください。`kimi` と `glm` selector は、それぞれ Moonshot AI と Zhipu AI の endpoint に接続し、各サービスの API key が必要です。現在の Client API では、Anthropic の selector は `cluade` です。
Anthropic を使用する場合、Request に `MaxTokens` を設定する必要があります。

## クイックスタート：通常のチャット

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
	request.AddSystem("あなたは親切なアシスタントです。")
	request.AddUserMessage("Go 言語を一文で説明してください。")

	response, err := sdk.Chat(request)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response.Message.Content)
}
```

## Client context と HTTP 設定

context は Client に一度設定し、`Chat`、`ChatRaw`、`ChatWithTools` で共用します。未設定の場合、または `SetContext(nil)` を呼び出した場合は、Client は `context.Background()` を使用します。

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

HTTP client または timeout をカスタマイズする場合は `NewWithOptions` を使います。

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

`HTTPClient` が nil の場合は `http.DefaultClient` が使われます。`Timeout` が未設定または 0 以下の場合、デフォルトのリクエスト timeout は 5 秒です。context に deadline がある場合、provider はその deadline に従います。

## Request の構築

よく使うメッセージは、ネストした `Message` 構造体を直接組み立てずに追加できます。

```go
request := requestprotocol.NewRequest("deepseek-v4-pro")
request.AddSystem("あなたは親切なアシスタントです。")
request.AddUserMessage("杭州の天気を調べてください。")
```

初期メッセージを `NewRequest` に渡すこともできます。

```go
request := requestprotocol.NewRequest(
	"deepseek-v4-pro",
	requestprotocol.Message{Role: "user", Content: "こんにちは。"},
)
```

Function Tool の object schema では、`String`、`Number`、`Integer`、`Boolean` の簡易プロパティ構築関数を利用できます。必須項目には `.Required()` を付けます。

```go
request.AddFunctionTool(
	"get_weather",
	"指定された都市の現在の天気を取得します。",
	requestprotocol.NewObjectParameters(
		requestprotocol.String("location", "都市名").Required(),
		requestprotocol.String("unit", "温度単位"),
	),
)
```

ツール定義は `request.Tools` に追加されます。会話履歴を手動で管理する場合は、`AddAssistantMessage`、`AddToolResult`、汎用の `AddMessage` も使えます。

高度な Request 設定は、引き続きフィールドから直接指定できます。

```go
maxTokens := 256
request.MaxTokens = &maxTokens

temperature := 0.2
request.Temperature = &temperature
```

## ツールの登録と実装

ツールは `toolprotocol.ToolExecutor` を実装します。`Start` はモデルが返した JSON 引数を受け取り、空ではない `ToolResult.Content` を返します。対応する `ToolCallID` は SDK が元の tool call から設定します。

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
		return toolprotocol.ToolResult{}, fmt.Errorf("天気引数の解析: %w", err)
	}

	data, err := json.Marshal(map[string]any{
		"location":      input.Location,
		"weather":       "晴れ",
		"temperature_c": 26,
	})
	if err != nil {
		return toolprotocol.ToolResult{}, err
	}
	return toolprotocol.ToolResult{Content: string(data)}, nil
}
```

登録時は返された error を確認してください。

```go
if err := sdk.RegisterTool(WeatherTool{}); err != nil {
	return err
}
```

## 自動マルチターン・ツール呼び出し

Request にツール schema を定義し、Client に同名の executor を登録します。`ChatWithTools` は最初のリクエストを送信し、モデルが要求したツールを実行し、tool call ID に紐付けた結果を追加して、モデルが通常の回答を返すまでリクエストを繰り返します。

```go
request := requestprotocol.NewRequest("deepseek-v4-pro")
request.AddSystem("あなたは親切なアシスタントです。")
request.AddUserMessage("杭州の天気を調べて簡潔にまとめてください。")
request.AddFunctionTool(
	"get_weather",
	"指定された都市の現在の天気を取得します。",
	requestprotocol.NewObjectParameters(
		requestprotocol.String("location", "都市名").Required(),
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

複数のツールを使う場合は、ツールごとに `RegisterTool` と `AddFunctionTool` を呼び出します。schema のツール名と executor の `Name()` は一致させてください。ツール実行エラー、未登録ツール、tool call ID の欠落、空の結果は SDK error として返されます。

## Response と token 使用量

`Chat` と `ChatWithTools` はどちらも `responseprotocol.Response` を返します。`Message` は最初の choice に直接アクセスするためのフィールドで、すべての choice は `Choices` に保持されます。

```go
fmt.Println(response.Message.Content)
fmt.Println(response.Message.ToolCalls)

if response.Usage != nil {
	fmt.Println("入力 token:", response.Usage.InputTokens)
	fmt.Println("出力 token:", response.Usage.OutputTokens)
	fmt.Println("合計 token:", response.Usage.TotalTokens)
}

for _, round := range response.RoundUsages {
	if round.Usage != nil {
		fmt.Printf("%d ターン目: %+v\n", round.Round, *round.Usage)
	}
}
```

通常の `Chat` では `RoundUsages` に 1 回分が記録されます。`ChatWithTools` の `Usage` は会話内の全モデルリクエストの合計で、`RoundUsages` は各リクエストの使用量です。`Raw` には最後のターンの provider 生レスポンスが保持されますが、`Response` の JSON encode 時には出力されません。

## Provider を直接使う

`Client` を使わず、provider を直接呼び出すこともできます。その場合は各 provider method に `context.Context` を渡します。

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
	request.AddUserMessage("杭州の天気を調べてください。")
	request.AddFunctionTool(
		"get_weather",
		"指定された都市の現在の天気を取得します。",
		requestprotocol.NewObjectParameters(
			requestprotocol.String("location", "都市名").Required(),
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

`ChatWithToolResults` は 1 回分の tool call batch を処理して後続リクエストを送信します。自動マルチターン処理には Client の `ChatWithTools` を使ってください。

## エラー処理

SDK error は `*sdkerror.SDKError` として返され、provider、エラー種別、HTTP status、メッセージ、元の cause を持ちます。`errors.As` で判定できます。

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

エラー種別は `invalid_request`、`transport`、`api`、`decode`、`tool_execution` です。

## テストの実行

単体テストとシミュレーションテストを実行します。

```bash
go test ./...
```

実 API テストは環境変数を読み込み、key が未設定の場合は skip します。

| 環境変数 | テスト対象 |
| --- | --- |
| `DEEPSEEK_API_KEY` | DeepSeek チャットと実際のツール呼び出し |
| `QWEN_API_KEY` | Bailian の `qwen-plus`、`kimi-k2.6`、`glm-5` チャットとツール呼び出し |
| `GLM_API_KEY` | Zhipu AI 直結 GLM チャット |
| `KIMI_API_KEY` または `MOONSHOT_API_KEY` | Moonshot 直結 Kimi チャット |

PowerShell の例：

```powershell
$env:DEEPSEEK_API_KEY = "DeepSeek API key"
$env:QWEN_API_KEY = "Alibaba Cloud Bailian API key"
go test ./... -count=1
```

API key は環境変数またはローカル設定で管理し、リポジトリに commit しないでください。
