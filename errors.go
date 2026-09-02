package keepz

import (
	"encoding/json"
	"fmt"
)

type Error struct {
	Message        string `json:"message"`
	StatusCode     int    `json:"statusCode"`
	ExceptionGroup int    `json:"exceptionGroup,omitempty"`
	HTTPStatusCode int    `json:"-"`
	Body           []byte `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("keepz: %s (%d)", e.Message, e.StatusCode)
	}
	if e.HTTPStatusCode != 0 {
		return fmt.Sprintf("keepz: http status %d", e.HTTPStatusCode)
	}
	return "keepz: api error"
}

func decodeKeepzError(statusCode int, data []byte) error {
	var apiErr Error
	if json.Unmarshal(data, &apiErr) == nil && (apiErr.Message != "" || apiErr.StatusCode != 0) {
		apiErr.HTTPStatusCode = statusCode
		apiErr.Body = data
		return &apiErr
	}
	return &Error{HTTPStatusCode: statusCode, Body: data}
}
