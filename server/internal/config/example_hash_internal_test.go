package config

import (
	"strings"
	"testing"
)

// The bootstrap admin hash shipped in the examples is public, so startup
// must refuse it, the same way it refuses the example session secret.
func TestLoadSecuritySettings_RejectsExampleAdminPasswordHash(t *testing.T) {
	t.Setenv("SESSION_SECRET", "test-session-secret-0123456789abcdef")
	t.Setenv("ADMIN_PASSWORD_HASH", exampleAdminPasswordHash)

	err := loadSecuritySettings(&Configuration{})
	if err == nil || !strings.Contains(err.Error(), "ADMIN_PASSWORD_HASH is the example value") {
		t.Fatalf("err = %v, want the example-hash rejection", err)
	}
}
