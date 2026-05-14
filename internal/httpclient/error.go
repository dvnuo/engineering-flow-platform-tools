package httpclient

import "fmt"

type HTTPError struct {
	Code, Message, Hint string
	Status              int
}

func (e *HTTPError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func mapStatus(status int) string {
	switch status {
	case 401:
		return "auth_failed"
	case 403:
		return "permission_denied"
	case 404:
		return "not_found"
	case 409:
		return "conflict"
	case 429:
		return "rate_limited"
	}
	if status >= 500 {
		return "server_error"
	}
	return "invalid_args"
}
