package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pvmss/handlers"
)

// TestFormatMemoryGB tests the memory formatting function
func TestFormatMemoryGB(t *testing.T) {
	tests := []struct {
		name     string
		value    int64
		isBytes  bool
		expected string
	}{
		{
			name:     "2GB from bytes",
			value:    2147483648, // 2GB in bytes
			isBytes:  true,
			expected: "2 GB",
		},
		{
			name:     "2GB from MB",
			value:    2048,
			isBytes:  false,
			expected: "2 GB",
		},
		{
			name:     "512MB from MB",
			value:    512,
			isBytes:  false,
			expected: "512 MB",
		},
		{
			name:     "1.5GB from MB",
			value:    1536,
			isBytes:  false,
			expected: "1.5 GB",
		},
		{
			name:     "Zero",
			value:    0,
			isBytes:  false,
			expected: "0 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handlers.FormatMemoryGB(tt.value, tt.isBytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestBytesToGB tests bytes to GB conversion
func TestBytesToGB(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected int64
	}{
		{
			name:     "1GB",
			bytes:    1073741824,
			expected: 1,
		},
		{
			name:     "2GB",
			bytes:    2147483648,
			expected: 2,
		},
		{
			name:     "Zero",
			bytes:    0,
			expected: 0,
		},
		{
			name:     "512MB (less than 1GB)",
			bytes:    536870912,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handlers.BytesToGB(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMBToGB tests MB to GB conversion
func TestMBToGB(t *testing.T) {
	tests := []struct {
		name     string
		mb       int64
		expected int64
	}{
		{
			name:     "1GB",
			mb:       1024,
			expected: 1,
		},
		{
			name:     "2GB",
			mb:       2048,
			expected: 2,
		},
		{
			name:     "Zero",
			mb:       0,
			expected: 0,
		},
		{
			name:     "512MB (less than 1GB)",
			mb:       512,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handlers.MBToGB(tt.mb)
			assert.Equal(t, tt.expected, result)
		})
	}
}
