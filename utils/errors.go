package utils

import (
	"fmt"
	"runtime"

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
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("%s: %s (original: %v)", e.Type, e.Message, e.Original)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// ToGraphQLError converts AppError to gqlerror.Error
func (e *AppError) ToGraphQLError() *gqlerror.Error {
	msg := e.Message
	if e.UserMsg != "" {
		msg = e.UserMsg
	}
	
	return &gqlerror.Error{
		Message: msg,
		Extensions: map[string]interface{}{
			"code":     string(e.Type),
			"location": e.Location,
		},
	}
}

// NewError creates a new AppError
func NewError(errType ErrorType, message string, original error) *AppError {
	_, file, line, _ := runtime.Caller(1)
	location := fmt.Sprintf("%s:%d", file, line)
	
	return &AppError{
		Type:     errType,
		Message:  message,
		Original: original,
		Location:  location,
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



