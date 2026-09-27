package error

import "fmt"

type ErrorKind string

const (
	InvalidRequest ErrorKind = "invalid_request"
	Transport      ErrorKind = "transport"
	API            ErrorKind = "api"
	Decode         ErrorKind = "decode"
)

type SDKError struct {
	Provider   string
	Kind       ErrorKind
	StatusCode int
	Message    string
	Body       string
	Cause      error
}

func (e *SDKError) Error() string {
	message := e.Provider
	if e.Kind != "" {
		if message != "" {
			message += " "
		}
		message += string(e.Kind)
	}
	if e.StatusCode != 0 {
		message += fmt.Sprintf(" (HTTP %d)", e.StatusCode)
	}
	if e.Message != "" {
		message += ": " + e.Message
	}
	if e.Body != "" {
		message += ": " + e.Body
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *SDKError) Unwrap() error {
	return e.Cause
}
