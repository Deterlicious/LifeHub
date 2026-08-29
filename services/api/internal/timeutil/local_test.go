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

func TestResolveRecurringLocalWallTimeUsesDeterministicGapAndFoldPolicy(t *testing.T) {
	location, err := LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	gap, err := ResolveRecurringLocalWallTime("2026-03-08T02:30:00", location)
	if err != nil {
		t.Fatal(err)
	}
	if got := gap.In(location).Format("2006-01-02T15:04:05 -07:00"); got != "2026-03-08T03:30:00 -04:00" {
		t.Fatalf("gap=%s", got)
	}
	fold, err := ResolveRecurringLocalWallTime("2026-11-01T01:30:00", location)
	if err != nil {
		t.Fatal(err)
	}
	if got := fold.Format(time.RFC3339); got != "2026-11-01T05:30:00Z" {
		t.Fatalf("fold=%s", got)
	}
	normal, err := ResolveRecurringLocalWallTime("2026-02-10T09:15:00", location)
	if err != nil {
		t.Fatal(err)
	}
	if got := normal.In(location).Format("2006-01-02T15:04:05"); got != "2026-02-10T09:15:00" {
		t.Fatalf("normal=%s", got)
	}
}

func TestResolveRecurringLocalWallTimeShiftsAcrossSkippedDate(t *testing.T) {
	location, err := LoadLocation("Pacific/Apia")
	if err != nil {
		t.Fatal(err)
	}
	instant, err := ResolveRecurringLocalWallTime("2011-12-30T09:00:00", location)
	if err != nil {
		t.Fatal(err)
	}
	if got := instant.In(location).Format("2006-01-02T15:04:05 -07:00"); got != "2011-12-31T09:00:00 +14:00" {
		t.Fatalf("shifted=%s", got)
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

	_, springStart, springEnd, err := LocalDayWindow(time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC), location)
	if err != nil {
		t.Fatal(err)
	}
	if got := springEnd.Sub(springStart); got != 23*time.Hour {
		t.Fatalf("spring day duration = %s, want 23h", got)
	}

	_, fallStart, fallEnd, err := LocalDayWindow(time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC), location)
	if err != nil {
		t.Fatal(err)
	}
	if got := fallEnd.Sub(fallStart); got != 25*time.Hour {
		t.Fatalf("fall day duration = %s, want 25h", got)
	}
}

func TestLocalDayWindowStartsAtFirstRepresentableInstantAfterMidnightGap(t *testing.T) {
	tests := []struct {
		name      string
		timezone  string
		date      time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "Cairo",
			timezone:  "Africa/Cairo",
			date:      time.Date(2026, time.April, 24, 12, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.April, 23, 22, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.April, 24, 21, 0, 0, 0, time.UTC),
		},
		{
			name:      "Santiago",
			timezone:  "America/Santiago",
			date:      time.Date(2026, time.September, 6, 12, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.September, 6, 4, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.September, 7, 3, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			location, err := LoadLocation(test.timezone)
			if err != nil {
				t.Fatal(err)
			}
			date, start, end, err := LocalDayWindow(test.date, location)
			if err != nil {
				t.Fatal(err)
			}
			if date != test.date.Format(time.DateOnly) || !start.Equal(test.wantStart) || !end.Equal(test.wantEnd) {
				t.Fatalf("window = %s [%s,%s), want %s [%s,%s)", date, start, end, test.date.Format(time.DateOnly), test.wantStart, test.wantEnd)
			}
			startLocal := start.In(location)
			if startLocal.Format(time.DateOnly) != date || startLocal.Hour() != 1 {
				t.Fatalf("start local = %s, want first representable hour of %s", startLocal, date)
			}
			if previousDate := start.Add(-time.Nanosecond).In(location).Format(time.DateOnly); previousDate == date {
				t.Fatalf("instant before start still belongs to %s", date)
			}
			if nextDate := end.In(location).Format(time.DateOnly); nextDate == date {
				t.Fatalf("exclusive end still belongs to %s", date)
			}
		})
	}
}

func TestLocalDateStartRejectsWhollySkippedDate(t *testing.T) {
	location, err := LoadLocation("Pacific/Apia")
	if err != nil {
		t.Fatal(err)
	}
	skipped := time.Date(2011, time.December, 30, 0, 0, 0, 0, time.UTC)
	if _, err := LocalDateStart(skipped, location); !errors.Is(err, ErrSkippedLocalDate) {
		t.Fatalf("LocalDateStart error = %v, want %v", err, ErrSkippedLocalDate)
	}
	if _, err := LocalDateEnd(skipped, location); !errors.Is(err, ErrSkippedLocalDate) {
		t.Fatalf("LocalDateEnd error = %v, want %v", err, ErrSkippedLocalDate)
	}

	dayBefore := skipped.AddDate(0, 0, -1)
	end, err := LocalDateEnd(dayBefore, location)
	if err != nil {
		t.Fatal(err)
	}
	dayAfterStart, err := LocalDateStart(skipped.AddDate(0, 0, 1), location)
	if err != nil {
		t.Fatal(err)
	}
	if !end.Equal(dayAfterStart) {
		t.Fatalf("day before end = %s, day after start = %s", end, dayAfterStart)
	}
}

func TestLocalDateStartSupportsGoZeroDate(t *testing.T) {
	date := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	start, err := LocalDateStart(date, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if !start.Equal(date) {
		t.Fatalf("start = %s, want %s", start, date)
	}
}
