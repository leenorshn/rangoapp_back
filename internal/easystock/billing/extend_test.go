package billing

import (
	"testing"
	"time"
)

func TestExtendExpiryUsesLaterBoundary(t *testing.T) {
	approved := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	future := approved.Add(48 * time.Hour)
	start, end := ExtendExpiry(&future, approved, 1)
	if !start.Equal(future) {
		t.Fatalf("start %s", start)
	}
	if !end.Equal(future.AddDate(0, 1, 0)) {
		t.Fatalf("end %s", end)
	}
	past := approved.Add(-48 * time.Hour)
	start, end = ExtendExpiry(&past, approved, 1)
	if !start.Equal(approved) || !end.Equal(approved.AddDate(0, 1, 0)) {
		t.Fatalf("past start %s end %s", start, end)
	}
}
