package client

import (
	"os"
	"testing"
)

func TestDeepSeekChat(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY is not set")
	}

	c, err := New("deepseek", apiKey)
	if err != nil {
		t.Fatal(err)
	}

	result, err := c.Chat("你好", "deepseek-v4-flash")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(result))
}

func TestNewUnsupportedSeries(t *testing.T) {
	_, err := New("unknown", "test-api-key")
	if err == nil {
		t.Fatal("expected an error for unsupported series")
	}
}
