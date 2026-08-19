package timeutil

import (
	"errors"
	"testing"
	"time"
)

func TestParseLocalWallTimeRejectsDSTGapAndFold(t *testing.T) {
	location, err := LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		value string
		want  error
	}{
		{name: "gap", value: "2026-03-08T02:30", want: ErrNonexistentTime},
		{name: "fold", value: "2026-11-01T01:30", want: ErrAmbiguousLocalTime},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseLocalWallTime(test.value, location)
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

func TestParseLocalWallTimeJakarta(t *testing.T) {
	location, err := LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseLocalWallTime("2026-08-19T14:30", location)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.August, 19, 7, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestLoadLocationRejectsProcessLocalTimezone(t *testing.T) {
	if _, err := LoadLocation("Local"); err == nil {
		t.Fatal("special Local timezone was accepted")
	}
}

func TestLocalDayWindowTracksDST(t *testing.T) {
	location, err := LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	_, springStart, springEnd := LocalDayWindow(time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC), location)
	if got := springEnd.Sub(springStart); got != 23*time.Hour {
		t.Fatalf("spring day duration = %s, want 23h", got)
	}

	_, fallStart, fallEnd := LocalDayWindow(time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC), location)
	if got := fallEnd.Sub(fallStart); got != 25*time.Hour {
		t.Fatalf("fall day duration = %s, want 25h", got)
	}
}
