package deepseek

import (
	"context"
	"os"
	"testing"
	"time"

	link "github.com/nuka-del/nuka-llm/Link"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
)

func TestDeepSeekLiveChat(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY is not set")
	}

	provider := NewWithOptions(apiKey, link.Options{Timeout: 60 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	maxTokens := 256
	request := requestprotocol.NewRequest("deepseek-v4-pro")
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
		finishReason := ""
		if len(response.Choices) > 0 {
			finishReason = response.Choices[0].FinishReason
		}
		t.Fatalf("response message is empty (finish_reason=%q)", finishReason)
	}
	t.Logf("DeepSeek raw response JSON: %s", response.Raw)
}
