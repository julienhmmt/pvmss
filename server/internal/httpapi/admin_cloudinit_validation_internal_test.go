package httpapi

import (
	"pvmss/server/internal/catalog"
	"strings"
	"testing"
)

func TestCloudInitTemplateValidationError(t *testing.T) {
	t.Parallel()

	_, err := catalog.CreateCloudInitTemplate(t.Context(), nil, "c", "web", "#cloud-config\nusers: [\n")
	code, message := cloudInitTemplateError(err)

	if code != "invalid_yaml" || !strings.Contains(message, "line") {
		t.Errorf("bad yaml -> %s %q, want invalid_yaml naming the line", code, message)
	}

	_, err = catalog.CreateCloudInitTemplate(t.Context(), nil, "c", "web", "users: []")
	if code, message = cloudInitTemplateError(err); code != "invalid_content" || !strings.Contains(message, "#cloud-config") {
		t.Errorf("no header -> %s %q", code, message)
	}
}
