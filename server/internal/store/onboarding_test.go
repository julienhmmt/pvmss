package store_test

import (
	"context"
	"testing"
)

//nolint:paralleltest // serial: shared database fixture
func TestOnboardingDone_MarkAndRead(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	done, err := st.OnboardingDoneSteps(ctx)
	if err != nil || len(done) != 0 {
		t.Fatalf("OnboardingDoneSteps empty = %v, err %v; want empty set", done, err)
	}

	if err := st.MarkOnboardingDone(ctx, "admin", []string{"nodes", "policy"}); err != nil {
		t.Fatalf("MarkOnboardingDone: %v", err)
	}

	done, err = st.OnboardingDoneSteps(ctx)
	if err != nil {
		t.Fatalf("OnboardingDoneSteps: %v", err)
	}

	for _, step := range []string{"nodes", "policy"} {
		if _, ok := done[step]; !ok {
			t.Fatalf("step %q missing from %v", step, done)
		}
	}

	// Re-marking is idempotent and does not duplicate the step.
	if err := st.MarkOnboardingDone(ctx, "admin", []string{"nodes"}); err != nil {
		t.Fatalf("MarkOnboardingDone remark: %v", err)
	}

	done, err = st.OnboardingDoneSteps(ctx)
	if err != nil || len(done) != 2 {
		t.Fatalf("OnboardingDoneSteps = %v, err %v; want 2 steps", done, err)
	}
}
