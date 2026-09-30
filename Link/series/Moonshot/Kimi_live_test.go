package moonshot

import (
	"context"
	"os"
	"testing"
	"time"

	link "github.com/nuka-del/nuka-llm/Link"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
)

func TestMoonshotKimiLiveChat(t *testing.T) {
	apiKey := os.Getenv("KIMI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("MOONSHOT_API_KEY")
	}
	if apiKey == "" {
		t.Skip("KIMI_API_KEY or MOONSHOT_API_KEY is not set")
	}

	provider := NewWithOptions(apiKey, link.Options{Timeout: 60 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	maxTokens := 32
	request := requestprotocol.NewRequest("kimi-k3")
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
}
