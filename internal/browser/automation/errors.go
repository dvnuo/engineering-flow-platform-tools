package automation

import "engineering-flow-platform-tools/internal/browser/probe"

type Error struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func NewError(code, message, hint string, status int) *Error {
	if status == 0 {
		status = 500
	}
	return &Error{
		Code:    code,
		Message: probe.RedactErrorMessage(message),
		Hint:    hint,
		Status:  status,
	}
}

func fail(code, message, hint string, status int) *Error {
	return NewError(code, message, hint, status)
}

func invalidArgs(message, hint string) *Error {
	return NewError("invalid_args", message, hint, 400)
}
