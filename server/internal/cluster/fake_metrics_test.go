package cluster

import (
	"context"
	"errors"
	"testing"
)

func TestFake_GetMetricsHistory_UnknownVM(t *testing.T) {
	t.Parallel()

	fake := NewFake("")

	if _, err := fake.GetMetricsHistory(context.Background(), "node01", 999999, MetricsTimeframeHour); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
