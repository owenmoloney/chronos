package cron

import (
	"testing"
	"time"
)

func TestIsDue(t *testing.T) {
	now := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	expr := "0 * * * *"
	tz := "UTC"

	cases := []struct {
		name           string
		lastEnqueuedAt time.Time
		wantDue        bool
	}{
		{"never run", time.Time{}, true},
		{"this window", now.Add(-90 * time.Minute), true},
		{"not yet", now.Add(-1 * time.Minute), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			due, err := IsDue(expr, tz, tc.lastEnqueuedAt, now)
			if err != nil {
				t.Fatalf("IsDue: %v", err)
			}
			if due != tc.wantDue {
				t.Fatalf("due = %v, want %v (last=%v now=%v)", due, tc.wantDue, tc.lastEnqueuedAt, now)
			}
		})
	}
}