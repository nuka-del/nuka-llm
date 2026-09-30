package alibaba

import (
	"context"
	"os"
	"testing"
	"time"

	link "github.com/nuka-del/nuka-llm/Link"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
)

func TestBailianModelsLiveChat(t *testing.T) {
	apiKey := os.Getenv("QWEN_API_KEY")
	if apiKey == "" {
		t.Skip("QWEN_API_KEY is not set")
	}

	models := []string{"qwen-plus", "kimi-k2.6", "glm-5"}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			provider := NewWithOptions(apiKey, link.Options{Timeout: 90 * time.Second})
			ctx, cancel := context.WithTimeout(context.Background(), 105*time.Second)
			defer cancel()
			maxTokens := 32
			request := requestprotocol.NewRequest(model)
			request.AddUserMessage("只回复 OK。")
			request.MaxTokens = &maxTokens

			raw, err := provider.ChatRaw(ctx, request)
			if err != nil {
				t.Fatalf("ChatRaw() error = %v", err)
			}
			response, err := provider.DecodeResponse(raw)
			if err != nil {
				t.Fatalf("DecodeResponse() error = %v", err)
			}
			if response.Message.Content == "" {
				t.Fatalf("response message is empty: %#v", response)
			}
			t.Logf("%s raw response JSON: %s", model, response.Raw)
		})
	}
}
