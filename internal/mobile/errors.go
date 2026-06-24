package mobile

import "fmt"

type Error struct {
	Code              string
	Message           string
	Hint              string
	Status            int
	Retryable         bool
	RecommendedAction string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewError(code, message, hint string, status int) *Error {
	return &Error{Code: code, Message: message, Hint: hint, Status: status}
}

func RetryableError(code, message, hint, action string, status int) *Error {
	return &Error{Code: code, Message: message, Hint: hint, Status: status, Retryable: true, RecommendedAction: action}
}
