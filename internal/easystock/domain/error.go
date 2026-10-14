package domain

import "errors"

const (
	CodeUnauthenticated    = "unauthenticated"
	CodePermissionDenied   = "permission_denied"
	CodeInvalid            = "invalid_argument"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodeFailedPrecondition = "failed_precondition"
	CodeAlreadyExists      = "already_exists"
	CodeInternal           = "internal"
)

// Error is safe to show to a client: Message is French and contains no SQL.
type Error struct {
	Code    string
	Reason  string
	Message string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Reason
}

// Extensions exposes a stable business code to GraphQL clients.
func (e *Error) Extensions() map[string]interface{} {
	code := e.Reason
	if code == "" {
		code = e.Code
	}
	return map[string]interface{}{"code": code}
}

func As(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

func New(code, reason, message string) *Error {
	return &Error{Code: code, Reason: reason, Message: message}
}

func Invalid(reason, message string) *Error {
	return New(CodeInvalid, reason, message)
}

func PermissionDenied(reason, message string) *Error {
	return New(CodePermissionDenied, reason, message)
}

func NotFound(message string) *Error {
	return New(CodeNotFound, "not_found", message)
}

func Conflict(reason, message string) *Error {
	return New(CodeConflict, reason, message)
}

func FailedPrecondition(reason, message string) *Error {
	return New(CodeFailedPrecondition, reason, message)
}

func AlreadyExists(message string) *Error {
	return New(CodeAlreadyExists, "already_exists", message)
}

func Unauthenticated(message string) *Error {
	return New(CodeUnauthenticated, "unauthenticated", message)
}

func Internal(message string) *Error {
	return New(CodeInternal, "internal", message)
}
