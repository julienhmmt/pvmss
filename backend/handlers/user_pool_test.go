package handlers

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSanitizeID tests the username sanitization function
func TestSanitizeID(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
		desc     string
	}{
		{"jhmt_admin", "jhmt_admin", "Simple username"},
		{"jhmt_admin@pve", "jhmt_admin", "Username with @pve suffix - CRITICAL: should strip for pool ID"},
		{"John Doe", "john_doe", "Username with spaces"},
		{"Test User@PVE", "test_user", "Mixed case with spaces and @pve suffix"},
		{"  spaced  ", "spaced", "Username with leading/trailing spaces"},
		{"UPPERCASE", "uppercase", "All uppercase"},
		{"user-with-dash", "user-with-dash", "Username with dashes"},
		{"user.with.dots", "user.with.dots", "Username with dots"},
		{"user_with_underscores", "user_with_underscores", "Username with underscores"},
		{"user@other.realm", "user", "Username with different realm - should strip for pool ID"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := sanitizeID(tc.input)
			assert.Equal(t, tc.expected, result, "sanitizeID(%q) should return %q", tc.input, tc.expected)
		})
	}
}

// TestCreateUserPoolSelf_AtPveSuffixHandling tests the critical @pve suffix handling
func TestCreateUserPoolSelf_AtPveSuffixHandling(t *testing.T) {
	testCases := []struct {
		sessionUsername string
		expectedPoolID  string
		expectedUserID  string
		desc            string
	}{
		{
			sessionUsername: "jhmt_admin@pve",
			expectedPoolID:  "pvmss_jhmt_admin",
			expectedUserID:  "jhmt_admin@pve",
			desc:            "Username with @pve suffix: strip for pool ID, preserve for user ID",
		},
		{
			sessionUsername: "jhmt_admin",
			expectedPoolID:  "pvmss_jhmt_admin",
			expectedUserID:  "jhmt_admin@pve",
			desc:            "Username without @pve: add @pve for user ID",
		},
		{
			sessionUsername: "test.user@pve",
			expectedPoolID:  "pvmss_test.user",
			expectedUserID:  "test.user@pve",
			desc:            "Username with dots and @pve: strip @pve for pool ID only",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Test pool ID generation (sanitizeID strips @pve)
			sanitizedUsername := sanitizeID(tc.sessionUsername)
			poolID := "pvmss_" + sanitizedUsername
			assert.Equal(t, tc.expectedPoolID, poolID, "Pool ID should be correctly generated (without @pve)")

			// Test user ID generation (logic from CreateUserPoolSelf lines 770-773)
			userID := tc.sessionUsername
			if !strings.Contains(userID, "@") {
				userID = userID + "@pve"
			}
			assert.Equal(t, tc.expectedUserID, userID, "User ID should be correctly generated (with @pve suffix)")
		})
	}
}

// TestPoolStatusDetection tests pool status detection logic
func TestPoolStatusDetection(t *testing.T) {
	testCases := []struct {
		name            string
		sessionUser     string
		existingPools   []string
		expectedHasPool bool
		expectedPool    string
	}{
		{
			name:            "User with existing pool",
			sessionUser:     "jhmt_admin@pve",
			existingPools:   []string{"pvmss_jhmt_admin", "pvmss_other_user"},
			expectedHasPool: true,
			expectedPool:    "pvmss_jhmt_admin",
		},
		{
			name:            "User without pool",
			sessionUser:     "jhmt_admin@pve",
			existingPools:   []string{"pvmss_other_user", "pvmss_another_user"},
			expectedHasPool: false,
			expectedPool:    "pvmss_jhmt_admin",
		},
		{
			name:            "User without @pve suffix",
			sessionUser:     "jhmt_admin",
			existingPools:   []string{"pvmss_jhmt_admin"},
			expectedHasPool: true,
			expectedPool:    "pvmss_jhmt_admin",
		},
		{
			name:            "No user in session",
			sessionUser:     "",
			existingPools:   []string{"pvmss_jhmt_admin"},
			expectedHasPool: false,
			expectedPool:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the pool status detection logic
			if tc.sessionUser == "" {
				assert.Equal(t, "", tc.expectedPool, "No user should result in empty pool name")
				return
			}

			// Sanitize username to match pool naming convention
			sanitizedUsername := sanitizeID(tc.sessionUser)
			expectedPoolName := "pvmss_" + sanitizedUsername

			assert.Equal(t, tc.expectedPool, expectedPoolName, "Pool name should be correctly generated")

			// Check if user's pool exists in the fetched pools
			hasPool := false
			for _, pool := range tc.existingPools {
				if pool == expectedPoolName {
					hasPool = true
					break
				}
			}

			assert.Equal(t, tc.expectedHasPool, hasPool, "Pool existence detection should be correct")
		})
	}
}

// TestCreateUserPoolSelf_URLValues tests URL form data handling
func TestCreateUserPoolSelf_URLValues(t *testing.T) {
	// Test that URL.Values are correctly constructed for POST requests
	formData := url.Values{}
	formData.Set("csrf_token", "test-token")

	assert.Equal(t, "test-token", formData.Get("csrf_token"), "CSRF token should be set correctly")
	assert.Equal(t, "csrf_token=test-token", formData.Encode(), "Form data should encode correctly")
}

// TestUserPoolSelfCreation_Idempotency tests idempotency logic
func TestUserPoolSelfCreation_Idempotency(t *testing.T) {
	// Test that the same pool ID is generated consistently
	username := "jhmt_admin@pve"

	// Generate pool ID multiple times
	poolID1 := "pvmss_" + sanitizeID(username)
	poolID2 := "pvmss_" + sanitizeID(username)
	poolID3 := "pvmss_" + sanitizeID(username)

	assert.Equal(t, poolID1, poolID2, "Pool ID generation should be consistent")
	assert.Equal(t, poolID2, poolID3, "Pool ID generation should be consistent")
	assert.Equal(t, "pvmss_jhmt_admin", poolID1, "Pool ID should be correctly formatted")
}
