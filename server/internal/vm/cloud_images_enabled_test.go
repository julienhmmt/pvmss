package vm

import (
	"errors"
	"pvmss/server/internal/auth"
	"testing"
)

// The image-mode code stays tested while cloud images are disabled in prod.
func init() { cloudImagesDisabled = false }

//nolint:paralleltest // flips the package-level cloudImagesDisabled switch
func TestCreate_Image_RejectedWhileDisabled(t *testing.T) {
	cloudImagesDisabled = true

	t.Cleanup(func() { cloudImagesDisabled = false })

	_, err := Create(t.Context(), auth.Identity{}, "default", CreateRequest{Image: &ImageRequest{}}, CreateDeps{})
	if !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("err = %v, want ErrInvalidSource", err)
	}
}
