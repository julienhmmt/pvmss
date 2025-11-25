package utils

import (
	"fmt"

	"pvmss/logger"
)

// ErrorWrapper provides contextual error wrapping with automatic logging
type ErrorWrapper struct {
	log logger.Logger
}

// NewErrorWrapper creates a new ErrorWrapper with the given logger
func NewErrorWrapper(l logger.Logger) *ErrorWrapper {
	return &ErrorWrapper{log: l}
}

// Wrap wraps an error with context and logs it
// Returns nil if err is nil
func (e *ErrorWrapper) Wrap(err error, msg string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	// Build the context message
	contextMsg := fmt.Sprintf(msg, args...)

	// Wrap the error with context
	wrapped := fmt.Errorf("%s: %w", contextMsg, err)

	// Log the error with context
	e.log.Error().
		Err(err).
		Str("context", contextMsg).
		Msg("Error occurred")

	return wrapped
}

// WrapWithFields wraps an error with context and additional structured fields
func (e *ErrorWrapper) WrapWithFields(err error, msg string, fields map[string]interface{}, args ...interface{}) error {
	if err == nil {
		return nil
	}

	// Build the context message
	contextMsg := fmt.Sprintf(msg, args...)

	// Wrap the error with context
	wrapped := fmt.Errorf("%s: %w", contextMsg, err)

	// Log with structured fields
	event := e.log.Error().Err(err).Str("context", contextMsg)
	for k, v := range fields {
		event = event.Interface(k, v)
	}
	event.Msg("Error occurred")

	return wrapped
}

// WrapSimple wraps an error without logging (for cases where logging is handled separately)
func WrapSimple(err error, msg string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	contextMsg := fmt.Sprintf(msg, args...)
	return fmt.Errorf("%s: %w", contextMsg, err)
}

// Must panics if err is not nil (useful for initialization code)
func Must(err error, msg string, args ...interface{}) {
	if err != nil {
		panic(fmt.Sprintf(msg+": %v", append(args, err)...))
	}
}
