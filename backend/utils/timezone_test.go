package utils

import (
	"testing"
	"time"
)

func TestGetAppLocation(t *testing.T) {
	// Test with UTC (most common case)
	t.Setenv("TZ", "UTC")
	loc := GetAppLocation()

	if loc == nil {
		t.Fatal("GetAppLocation() returned nil")
	}

	// Verify it returns UTC when TZ is set to UTC
	if loc.String() != "UTC" {
		t.Errorf("GetAppLocation() with TZ=UTC = %v, want UTC", loc.String())
	}
}

func TestGetAppLocationSingleton(t *testing.T) {
	// Test that GetAppLocation returns the same instance (singleton)
	t.Setenv("TZ", "UTC")

	loc1 := GetAppLocation()
	loc2 := GetAppLocation()

	if loc1 != loc2 {
		t.Error("GetAppLocation() should return the same instance (singleton)")
	}
}

func TestGetAppLocationWithTime(t *testing.T) {
	// Test that the location works correctly with time operations
	t.Setenv("TZ", "UTC")

	loc := GetAppLocation()
	now := time.Now().In(loc)

	// Verify the location is applied correctly
	if now.Location() != loc {
		t.Error("Time.In() did not apply the location correctly")
	}
}

// Note: Tests for empty TZ and invalid TZ are not possible with the current
// singleton pattern. Once GetAppLocation() is initialized (sync.Once),
// subsequent calls return the cached instance regardless of environment changes.
// To properly test these scenarios, we would need to:
// 1. Refactor GetAppLocation() to accept an optional parameter for testing
// 2. Use subprocess-based tests to test different TZ values in isolation
// 3. Reset the singleton between tests (not recommended for production code)
