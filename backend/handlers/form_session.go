package handlers

import (
	"context"
	"strings"

	"github.com/alexedwards/scs/v2"
)

// FormSession provides utilities for preserving and restoring form data across requests
// This is useful for validation errors where we want to show the form again with previous values
type FormSession struct {
	sessionManager *scs.SessionManager
	ctx            context.Context
	formName       string
}

// NewFormSession creates a new FormSession helper
func NewFormSession(sessionManager *scs.SessionManager, ctx context.Context, formName string) *FormSession {
	return &FormSession{
		sessionManager: sessionManager,
		ctx:            ctx,
		formName:       formName,
	}
}

// PreserveFormData stores form data in session for later retrieval
func (fs *FormSession) PreserveFormData(data map[string]interface{}) {
	if fs.sessionManager == nil {
		return
	}
	key := fs.formName + "_data"
	fs.sessionManager.Put(fs.ctx, key, data)
}

// PreserveErrors stores validation errors in session
func (fs *FormSession) PreserveErrors(errors []string) {
	if fs.sessionManager == nil || len(errors) == 0 {
		return
	}
	key := fs.formName + "_errors"
	fs.sessionManager.Put(fs.ctx, key, strings.Join(errors, "; "))
}

// PreserveFormWithErrors stores both form data and errors
func (fs *FormSession) PreserveFormWithErrors(data map[string]interface{}, errors []string) {
	fs.PreserveFormData(data)
	fs.PreserveErrors(errors)
}

// RestoreFormData retrieves previously stored form data
// Returns the data map and true if found, or nil and false if not found
func (fs *FormSession) RestoreFormData() (map[string]interface{}, bool) {
	if fs.sessionManager == nil {
		return nil, false
	}

	key := fs.formName + "_data"
	data := fs.sessionManager.Get(fs.ctx, key)

	// Remove from session after retrieval (single use)
	fs.sessionManager.Remove(fs.ctx, key)

	if dataMap, ok := data.(map[string]interface{}); ok {
		return dataMap, true
	}
	return nil, false
}

// RestoreErrors retrieves previously stored validation errors
// Returns the error string and true if found, or empty string and false if not found
func (fs *FormSession) RestoreErrors() (string, bool) {
	if fs.sessionManager == nil {
		return "", false
	}

	key := fs.formName + "_errors"
	errors := fs.sessionManager.Get(fs.ctx, key)

	// Remove from session after retrieval (single use)
	fs.sessionManager.Remove(fs.ctx, key)

	if errStr, ok := errors.(string); ok && errStr != "" {
		return errStr, true
	}
	return "", false
}

// RestoreAll retrieves both form data and errors
func (fs *FormSession) RestoreAll() (data map[string]interface{}, errors string, found bool) {
	data, hasData := fs.RestoreFormData()
	errors, hasErrors := fs.RestoreErrors()
	found = hasData || hasErrors
	return
}

// Clear removes all stored form data and errors
func (fs *FormSession) Clear() {
	if fs.sessionManager == nil {
		return
	}
	fs.sessionManager.Remove(fs.ctx, fs.formName+"_data")
	fs.sessionManager.Remove(fs.ctx, fs.formName+"_errors")
}

// PreserveField stores a single field value in session
func (fs *FormSession) PreserveField(fieldName string, value interface{}) {
	if fs.sessionManager == nil {
		return
	}
	key := fs.formName + "_field_" + fieldName
	fs.sessionManager.Put(fs.ctx, key, value)
}

// RestoreField retrieves a single field value from session
func (fs *FormSession) RestoreField(fieldName string) (interface{}, bool) {
	if fs.sessionManager == nil {
		return nil, false
	}

	key := fs.formName + "_field_" + fieldName
	value := fs.sessionManager.Get(fs.ctx, key)

	// Remove from session after retrieval
	fs.sessionManager.Remove(fs.ctx, key)

	if value != nil {
		return value, true
	}
	return nil, false
}

// FormSessionHelper provides a convenient way to access FormSession from HandlerContext
func (ctx *HandlerContext) FormSession(formName string) *FormSession {
	return NewFormSession(ctx.SessionManager, ctx.Request.Context(), formName)
}
