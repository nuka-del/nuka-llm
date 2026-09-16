package link

import (
	"context"

	request "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
)

type Link interface {
	Chat(ctx context.Context, request request.Request) ([]byte, error)
	BuildRequest(request request.Request) ([]byte, error)
}
