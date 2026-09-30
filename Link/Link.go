package link

import (
	"context"

	request "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
	response "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
	toolprotocol "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type Link interface {
	ChatRaw(ctx context.Context, request request.Request) ([]byte, error)
	BuildRequest(request request.Request) ([]byte, error)
	DecodeResponse(raw []byte) (response.Response, error)
	RegisterTool(executor toolprotocol.ToolExecutor) error
	ChatWithToolResults(
		ctx context.Context,
		request *request.Request,
		firstResponse response.Response,
	) (response.Response, error)
}
