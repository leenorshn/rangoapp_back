package utils

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

// ErrorType represents the type of error
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "VALIDATION_ERROR"
	ErrorTypeNotFound     ErrorType = "NOT_FOUND"
	ErrorTypeUnauthorized ErrorType = "UNAUTHORIZED"
	ErrorTypeForbidden    ErrorType = "FORBIDDEN"
	ErrorTypeInternal     ErrorType = "INTERNAL_ERROR"
	ErrorTypeDatabase     ErrorType = "DATABASE_ERROR"
	ErrorTypeConflict     ErrorType = "CONFLICT"
)

// AppError represents an application error with context
type AppError struct {
	Type      ErrorType
	Message   string
	Original  error
	Location  string
	UserMsg   string // User-friendly message
	Stack     string // Stack trace
}

// Error implements the error interface
func (e *AppError) Error() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("[%s] %s", e.Type, e.Message))
	
	if e.Location != "" {
		parts = append(parts, fmt.Sprintf("at %s", e.Location))
	}
	
	if e.Original != nil {
		parts = append(parts, fmt.Sprintf("(original: %v)", e.Original))
	}
	
	return strings.Join(parts, " ")
}

// Unwrap returns the original error for error unwrapping
func (e *AppError) Unwrap() error {
	return e.Original
}

// ToGraphQLError converts AppError to gqlerror.Error
func (e *AppError) ToGraphQLError() *gqlerror.Error {
	msg := e.Message
	if e.UserMsg != "" {
		msg = e.UserMsg
	}
	
	extensions := map[string]interface{}{
		"code":     string(e.Type),
		"location": e.Location,
	}
	
	// Include stack trace in development mode only
	if os.Getenv("ENV") == "development" || os.Getenv("ENV") == "dev" {
		if e.Stack != "" {
			extensions["stack"] = e.Stack
		}
	}
	
	return &gqlerror.Error{
		Message:    msg,
		Extensions: extensions,
	}
}

// getStackTrace captures a stack trace (up to 10 frames)
func getStackTrace(skip int) string {
	var stack []string
	for i := 0; i < 10; i++ {
		pc, file, line, ok := runtime.Caller(skip + i)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			funcName := fn.Name()
			// Simplify function names
			if idx := strings.LastIndex(funcName, "/"); idx >= 0 {
				funcName = funcName[idx+1:]
			}
			stack = append(stack, fmt.Sprintf("%s:%d %s", file, line, funcName))
		}
	}
	return strings.Join(stack, "\n")
}

// NewError creates a new AppError
func NewError(errType ErrorType, message string, original error) *AppError {
	_, file, line, _ := runtime.Caller(1)
	location := fmt.Sprintf("%s:%d", file, line)
	stack := getStackTrace(1)
	
	return &AppError{
		Type:     errType,
		Message:  message,
		Original: original,
		Location: location,
		Stack:    stack,
	}
}

// NewErrorWithUserMsg creates a new AppError with a user-friendly message
func NewErrorWithUserMsg(errType ErrorType, message, userMsg string, original error) *AppError {
	err := NewError(errType, message, original)
	err.UserMsg = userMsg
	return err
}

// WrapError wraps an existing error with context
func WrapError(err error, message string) *AppError {
	if err == nil {
		return nil
	}
	
	// If it's already an AppError, just add context
	if appErr, ok := err.(*AppError); ok {
		appErr.Message = fmt.Sprintf("%s: %s", message, appErr.Message)
		return appErr
	}
	
	return NewError(ErrorTypeInternal, fmt.Sprintf("%s: %v", message, err), err)
}

// NewValidationError creates a validation error
func NewValidationError(message string) *AppError {
	return NewErrorWithUserMsg(ErrorTypeValidation, message, message, nil)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string) *AppError {
	msg := fmt.Sprintf("%s not found", resource)
	return NewErrorWithUserMsg(ErrorTypeNotFound, msg, msg, nil)
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "Unauthorized"
	}
	return NewErrorWithUserMsg(ErrorTypeUnauthorized, message, message, nil)
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "Access denied"
	}
	return NewErrorWithUserMsg(ErrorTypeForbidden, message, message, nil)
}

// NewDatabaseError creates a database error
func NewDatabaseError(operation string, original error) *AppError {
	msg := fmt.Sprintf("Database error during %s", operation)
	userMsg := "A database error occurred. Please try again later."
	return NewErrorWithUserMsg(ErrorTypeDatabase, msg, userMsg, original)
}

// NewConflictError creates a conflict error
func NewConflictError(message string) *AppError {
	return NewErrorWithUserMsg(ErrorTypeConflict, message, message, nil)
}

// HandleError converts an error to GraphQL error format
func HandleError(err error) error {
	if err == nil {
		return nil
	}
	
	// If it's already a gqlerror, return as is
	if gqlErr, ok := err.(*gqlerror.Error); ok {
		return gqlErr
	}
	
	// If it's an AppError, convert it
	if appErr, ok := err.(*AppError); ok {
		return appErr.ToGraphQLError()
	}
	
	// Otherwise, wrap it as an internal error
	appErr := NewDatabaseError("operation", err)
	return appErr.ToGraphQLError()
}

// FromGraphQLError converts a gqlerror to AppError
func FromGraphQLError(gqlErr *gqlerror.Error) *AppError {
	errType := ErrorTypeInternal
	if code, ok := gqlErr.Extensions["code"].(string); ok {
		errType = ErrorType(code)
	}
	
	location := ""
	if loc, ok := gqlErr.Extensions["location"].(string); ok {
		location = loc
	}
	
	return &AppError{
		Type:     errType,
		Message:  gqlErr.Message,
		UserMsg:  gqlErr.Message,
		Location: location,
		Stack:    getStackTrace(1),
	}
}

// WrapGraphQLError wraps a gqlerror with additional context
func WrapGraphQLError(gqlErr *gqlerror.Error, context string) *AppError {
	appErr := FromGraphQLError(gqlErr)
	appErr.Message = fmt.Sprintf("%s: %s", context, appErr.Message)
	return appErr
}

// Errorf creates a new AppError with formatted message (replacement for gqlerror.Errorf)
func Errorf(errType ErrorType, format string, args ...interface{}) *AppError {
	message := fmt.Sprintf(format, args...)
	return NewErrorWithUserMsg(errType, message, message, nil)
}

// ValidationErrorf creates a validation error with formatted message
func ValidationErrorf(format string, args ...interface{}) *AppError {
	return Errorf(ErrorTypeValidation, format, args...)
}

// NotFoundErrorf creates a not found error with formatted message
func NotFoundErrorf(format string, args ...interface{}) *AppError {
	return Errorf(ErrorTypeNotFound, format, args...)
}

// DatabaseErrorf creates a database error with formatted message
func DatabaseErrorf(operation string, format string, args ...interface{}) *AppError {
	message := fmt.Sprintf(format, args...)
	userMsg := "A database error occurred. Please try again later."
	return NewErrorWithUserMsg(ErrorTypeDatabase, fmt.Sprintf("Database error during %s: %s", operation, message), userMsg, nil)
}



