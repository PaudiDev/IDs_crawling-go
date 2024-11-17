package customerrors

import (
	"fmt"
)

type ErrorHttpResponse struct {
	code int
	msg  string
}

func (e ErrorHttpResponse) Error() string {
	return fmt.Sprintf("HTTP Response Error: %d - %s", e.code, e.msg)
}

var (
	ErrorUnauthorized = ErrorHttpResponse{401, "Unauthorized"}
	ErrorNotFound     = ErrorHttpResponse{404, "Not Found"}
	ErrorRateLimit    = ErrorHttpResponse{429, "Rate Limit Exceeded"}
)

func InferHttpError(code int) error {
	switch code {
	case 401:
		return ErrorUnauthorized
	case 404:
		return ErrorNotFound
	case 429:
		return ErrorRateLimit
	default:
		return ErrorHttpResponse{code, "unexpected response status code"}
	}
}

func MakeErrorHttpResponse(code int, msg string) error {
	return ErrorHttpResponse{code, msg}
}
