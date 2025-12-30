package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestAppError(t *testing.T) {
	t.Run("Error message format", func(t *testing.T) {
		err := NewError(ErrorTypeValidation, "test message", nil)
		errorStr := err.Error()
		assert.Contains(t, errorStr, "VALIDATION_ERROR")
		assert.Contains(t, errorStr, "test message")
	})

	t.Run("Error with original error", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := NewError(ErrorTypeDatabase, "database error", originalErr)
		errorStr := err.Error()
		assert.Contains(t, errorStr, "DATABASE_ERROR")
		assert.Contains(t, errorStr, "database error")
		assert.Contains(t, errorStr, "original error")
	})

	t.Run("Error location", func(t *testing.T) {
		err := NewError(ErrorTypeInternal, "test", nil)
		assert.Contains(t, err.Location, ".go")
		assert.NotEmpty(t, err.Location)
	})
}

func TestToGraphQLError(t *testing.T) {
	t.Run("Convert to GraphQL error", func(t *testing.T) {
		err := NewError(ErrorTypeValidation, "validation failed", nil)
		gqlErr := err.ToGraphQLError()
		
		assert.NotNil(t, gqlErr)
		assert.Equal(t, "validation failed", gqlErr.Message)
		assert.Equal(t, "VALIDATION_ERROR", gqlErr.Extensions["code"])
		assert.NotNil(t, gqlErr.Extensions["location"])
	})

	t.Run("Convert with user message", func(t *testing.T) {
		err := NewErrorWithUserMsg(ErrorTypeValidation, "technical message", "User-friendly message", nil)
		gqlErr := err.ToGraphQLError()
		
		assert.Equal(t, "User-friendly message", gqlErr.Message)
	})

	t.Run("Convert without user message", func(t *testing.T) {
		err := NewError(ErrorTypeNotFound, "not found", nil)
		gqlErr := err.ToGraphQLError()
		
		assert.Equal(t, "not found", gqlErr.Message)
	})
}

func TestNewError(t *testing.T) {
	t.Run("Create validation error", func(t *testing.T) {
		err := NewError(ErrorTypeValidation, "invalid input", nil)
		assert.Equal(t, ErrorTypeValidation, err.Type)
		assert.Equal(t, "invalid input", err.Message)
		assert.Nil(t, err.Original)
	})

	t.Run("Create error with original", func(t *testing.T) {
		original := errors.New("db connection failed")
		err := NewError(ErrorTypeDatabase, "database error", original)
		assert.Equal(t, ErrorTypeDatabase, err.Type)
		assert.Equal(t, original, err.Original)
	})
}

func TestNewErrorWithUserMsg(t *testing.T) {
	t.Run("Create error with user message", func(t *testing.T) {
		err := NewErrorWithUserMsg(ErrorTypeValidation, "tech msg", "user msg", nil)
		assert.Equal(t, "tech msg", err.Message)
		assert.Equal(t, "user msg", err.UserMsg)
	})
}

func TestWrapError(t *testing.T) {
	t.Run("Wrap nil error", func(t *testing.T) {
		err := WrapError(nil, "context")
		assert.Nil(t, err)
	})

	t.Run("Wrap regular error", func(t *testing.T) {
		original := errors.New("original")
		err := WrapError(original, "context")
		
		assert.NotNil(t, err)
		assert.Equal(t, ErrorTypeInternal, err.Type)
		assert.Equal(t, original, err.Original)
		assert.Contains(t, err.Message, "context")
	})

	t.Run("Wrap AppError", func(t *testing.T) {
		appErr := NewError(ErrorTypeValidation, "validation failed", nil)
		wrapped := WrapError(appErr, "additional context")
		
		assert.Equal(t, appErr, wrapped)
		assert.Contains(t, wrapped.Message, "additional context")
		assert.Contains(t, wrapped.Message, "validation failed")
	})
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("field is required")
	assert.Equal(t, ErrorTypeValidation, err.Type)
	assert.Equal(t, "field is required", err.Message)
	assert.Equal(t, "field is required", err.UserMsg)
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("User")
	assert.Equal(t, ErrorTypeNotFound, err.Type)
	assert.Contains(t, err.Message, "User")
	assert.Contains(t, err.Message, "not found")
}

func TestNewUnauthorizedError(t *testing.T) {
	t.Run("With message", func(t *testing.T) {
		err := NewUnauthorizedError("invalid credentials")
		assert.Equal(t, ErrorTypeUnauthorized, err.Type)
		assert.Equal(t, "invalid credentials", err.Message)
	})

	t.Run("Without message", func(t *testing.T) {
		err := NewUnauthorizedError("")
		assert.Equal(t, ErrorTypeUnauthorized, err.Type)
		assert.Equal(t, "Unauthorized", err.Message)
	})
}

func TestNewForbiddenError(t *testing.T) {
	t.Run("With message", func(t *testing.T) {
		err := NewForbiddenError("insufficient permissions")
		assert.Equal(t, ErrorTypeForbidden, err.Type)
		assert.Equal(t, "insufficient permissions", err.Message)
	})

	t.Run("Without message", func(t *testing.T) {
		err := NewForbiddenError("")
		assert.Equal(t, ErrorTypeForbidden, err.Type)
		assert.Equal(t, "Access denied", err.Message)
	})
}

func TestNewDatabaseError(t *testing.T) {
	original := errors.New("connection timeout")
	err := NewDatabaseError("findUser", original)
	
	assert.Equal(t, ErrorTypeDatabase, err.Type)
	assert.Contains(t, err.Message, "findUser")
	assert.Equal(t, original, err.Original)
	assert.Equal(t, "A database error occurred. Please try again later.", err.UserMsg)
}

func TestNewConflictError(t *testing.T) {
	err := NewConflictError("email already exists")
	assert.Equal(t, ErrorTypeConflict, err.Type)
	assert.Equal(t, "email already exists", err.Message)
}

func TestHandleError(t *testing.T) {
	t.Run("Handle nil error", func(t *testing.T) {
		result := HandleError(nil)
		assert.Nil(t, result)
	})

	t.Run("Handle gqlerror", func(t *testing.T) {
		gqlErr := &gqlerror.Error{
			Message: "GraphQL error",
		}
		result := HandleError(gqlErr)
		assert.Equal(t, gqlErr, result)
	})

	t.Run("Handle AppError", func(t *testing.T) {
		appErr := NewValidationError("validation failed")
		result := HandleError(appErr)
		
		gqlErr, ok := result.(*gqlerror.Error)
		assert.True(t, ok)
		assert.Equal(t, "validation failed", gqlErr.Message)
	})

	t.Run("Handle regular error", func(t *testing.T) {
		regularErr := errors.New("some error")
		result := HandleError(regularErr)
		
		gqlErr, ok := result.(*gqlerror.Error)
		assert.True(t, ok)
		assert.Equal(t, "DATABASE_ERROR", gqlErr.Extensions["code"])
	})
}

func TestErrorTypes(t *testing.T) {
	errorTypes := []ErrorType{
		ErrorTypeValidation,
		ErrorTypeNotFound,
		ErrorTypeUnauthorized,
		ErrorTypeForbidden,
		ErrorTypeInternal,
		ErrorTypeDatabase,
		ErrorTypeConflict,
	}

	for _, errType := range errorTypes {
		err := NewError(errType, "test", nil)
		assert.Equal(t, errType, err.Type)
	}
}



