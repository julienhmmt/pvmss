package vm

import (
	"testing"
	"time"
)

func TestExtendedExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	cases := []struct {
		name    string
		current time.Time
		days    int
		maxDays int
		want    time.Time
	}{
		{"expired lease extends from now", now.Add(-5 * day), 10, 30, now.Add(10 * day)},
		{"active lease adds onto current", now.Add(20 * day), 10, 30, now.Add(30 * day)},
		{"cap at now plus maxDays", now.Add(20 * day), 30, 30, now.Add(30 * day)},
		{"current beyond the cap stays unchanged", now.Add(60 * day), 10, 30, now.Add(60 * day)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := extendedExpiry(tc.current, now, tc.days, tc.maxDays)
			if !got.Equal(tc.want) {
				t.Fatalf("extendedExpiry = %v, want %v", got, tc.want)
			}

			if got.Before(tc.current) {
				t.Fatalf("extension shortened the lease: %v < %v", got, tc.current)
			}
		})
	}
}
