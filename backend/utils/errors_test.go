package utils

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapSimple(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		msg     string
		args    []interface{}
		wantNil bool
		wantMsg string
	}{
		{
			name:    "Wrap non-nil error",
			err:     errors.New("original error"),
			msg:     "context message",
			wantNil: false,
			wantMsg: "context message: original error",
		},
		{
			name:    "Wrap with formatted message",
			err:     errors.New("original error"),
			msg:     "context %s",
			args:    []interface{}{"formatted"},
			wantNil: false,
			wantMsg: "context formatted: original error",
		},
		{
			name:    "Wrap nil error",
			err:     nil,
			msg:     "context message",
			wantNil: true,
			wantMsg: "",
		},
		{
			name:    "Wrap nil error with args",
			err:     nil,
			msg:     "context %s",
			args:    []interface{}{"formatted"},
			wantNil: true,
			wantMsg: "",
		},
		{
			name:    "Empty message",
			err:     errors.New("original error"),
			msg:     "",
			wantNil: false,
			wantMsg: ": original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WrapSimple(tt.err, tt.msg, tt.args...)

			if tt.wantNil {
				if got != nil {
					t.Errorf("WrapSimple() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("WrapSimple() = nil, want error")
				}
				if got != nil && got.Error() != tt.wantMsg {
					t.Errorf("WrapSimple() error = %v, want %v", got.Error(), tt.wantMsg)
				}
			}
		})
	}
}

func TestMust(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		msg       string
		args      []interface{}
		wantPanic bool
	}{
		{
			name:      "Must with nil error",
			err:       nil,
			msg:       "should not panic",
			wantPanic: false,
		},
		{
			name:      "Must with non-nil error",
			err:       errors.New("error occurred"),
			msg:       "initialization failed",
			wantPanic: true,
		},
		{
			name:      "Must with formatted message",
			err:       errors.New("error occurred"),
			msg:       "failed to %s",
			args:      []interface{}{"initialize"},
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.wantPanic {
						t.Errorf("Must() panicked unexpectedly: %v", r)
					}
				} else {
					if tt.wantPanic {
						t.Error("Must() did not panic when it should have")
					}
				}
			}()

			Must(tt.err, tt.msg, tt.args...)
		})
	}
}

func TestWrapSimpleUnwrap(t *testing.T) {
	// Test that wrapped errors can be unwrapped
	originalErr := errors.New("original error")
	wrappedErr := WrapSimple(originalErr, "context")

	// Unwrap should return the original error
	if !errors.Is(wrappedErr, originalErr) {
		t.Error("Wrapped error should contain original error")
	}

	// Unwrap should return the original error
	unwrapped := errors.Unwrap(wrappedErr)
	if unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestErrorChaining(t *testing.T) {
	// Test that multiple wraps work correctly
	err1 := errors.New("error 1")
	err2 := WrapSimple(err1, "context 1")
	err3 := WrapSimple(err2, "context 2")

	expectedMsg := "context 2: context 1: error 1"
	if err3.Error() != expectedMsg {
		t.Errorf("Chained error message = %v, want %v", err3.Error(), expectedMsg)
	}

	// Verify we can unwrap to the original error
	if !errors.Is(err3, err1) {
		t.Error("Chained error should contain original error")
	}
}

func TestMustPanicMessage(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// Verify panic message format
			panicMsg, ok := r.(string)
			if !ok {
				t.Errorf("Panic message is not a string: %v", r)
				return
			}

			if !strings.HasPrefix(panicMsg, "initialization failed:") {
				t.Errorf("Panic message = %v, should start with 'initialization failed:'", panicMsg)
			}
		}
	}()

	Must(errors.New("error"), "initialization failed")
}
