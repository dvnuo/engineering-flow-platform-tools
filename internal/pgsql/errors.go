package pgsql

// Stable envelope error codes produced by this package. Commands map them
// straight onto output.Failure so agents can branch on error.code.
const (
	CodeReadOnlyViolation = "read_only_violation"
	CodeInvalidArgs       = "invalid_args"
	CodePermissionDenied  = "permission_denied"
	CodeQueryTimeout      = "query_timeout"
	CodeNetworkError      = "network_error"
	CodeAuthFailed        = "auth_failed"
	CodeNotSupported      = "not_supported"
	CodeServerError       = "server_error"
	CodeConfigError       = "config_error"
)

// Error is a classified pgsql failure: a stable code for the envelope, a
// human message that never contains credentials, an optional hint, an HTTP-ish
// status, and the PostgreSQL SQLSTATE when the server produced the error.
type Error struct {
	Code     string
	Message  string
	Hint     string
	Status   int
	SQLState string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

func violation(msg, hint string) *Error {
	return &Error{Code: CodeReadOnlyViolation, Message: msg, Hint: hint, Status: 403}
}

func invalidArgs(msg, hint string) *Error {
	return &Error{Code: CodeInvalidArgs, Message: msg, Hint: hint, Status: 400}
}
