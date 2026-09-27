package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	sdkerror "github.com/nuka-del/nuka-llm/Error"
	linkprotocol "github.com/nuka-del/nuka-llm/Link"
	requestprotocol "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
)

type testLink struct {
	rawCalls    int
	decodeCalls int
}

func (l *testLink) ChatRaw(context.Context, requestprotocol.Request) ([]byte, error) {
	l.rawCalls++
	return []byte(`{"choices":[]}`), nil
}

func (*testLink) BuildRequest(requestprotocol.Request) ([]byte, error) {
	return nil, nil
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

	response, err := client.Chat(context.Background(), requestprotocol.Request{})
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

	raw, err := client.ChatRaw(context.Background(), requestprotocol.Request{})
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
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

	response, err := sdk.Chat(context.Background(), requestprotocol.Request{
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

	_, err = sdk.ChatRaw(context.Background(), requestprotocol.Request{
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
