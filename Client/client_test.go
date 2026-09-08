package client

import (
	"encoding/json"
	"os"
	"testing"

	toolprotocol "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type testTool struct{}

func (testTool) Info() toolprotocol.ToolInfo {
	return toolprotocol.NewToolInfo(
		"test_tool",
		"测试工具",
	)
}

func (testTool) Start() (string, error) {
	return "success", nil
}

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

func TestRegisterToolDefinitions(t *testing.T) {
	c, err := New("deepseek", "test-api-key")
	if err != nil {
		t.Fatal(err)
	}

	c.RegisterTool(testTool{})

	data, err := c.ToolDefinitions()
	if err != nil {
		t.Fatal(err)
	}

	var definitions []toolprotocol.ToolInfo
	if err := json.Unmarshal(data, &definitions); err != nil {
		t.Fatal(err)
	}

	if len(definitions) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(definitions))
	}

	if definitions[0].Name != "test_tool" {
		t.Fatalf("unexpected tool name: %s", definitions[0].Name)
	}
}
