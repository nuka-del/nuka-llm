package link

import (
	request "github.com/nuka-del/nuka-llm/Protocol/Request_Protocol"
)

type Link interface {
	Chat(request request.Request) ([]byte, error)
	BuildRequest(request request.Request) ([]byte, error)
}
