package store

import (
	"errors"
	"testing"
	"time"
)

func TestCalculateReminderFireAtUsesStrictSourceShape(t *testing.T) {
	anchor := time.Date(2026, time.August, 25, 8, 30, 0, 0, time.UTC)
	minutes := 45
	got, err := calculateReminderFireAt(reminderDefinition{
		ScheduleKind: "before_moment", MinutesBefore: &minutes,
	}, reminderSource{Active: true, Moment: &anchor})
	if err != nil {
		t.Fatal(err)
	}
	want := anchor.Add(-45 * time.Minute)
	if !got.Equal(want) {
		t.Fatalf("fire_at = %s, want %s", got, want)
	}

	days := 2
	local := "09:15"
	date := time.Date(2026, time.August, 28, 0, 0, 0, 0, time.UTC)
	got, err = calculateReminderFireAt(reminderDefinition{
		ScheduleKind: "before_date", DaysBefore: &days, TimeLocal: &local,
	}, reminderSource{Active: true, Date: &date, Timezone: "Asia/Jakarta"})
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2026, time.August, 26, 2, 15, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("date fire_at = %s, want %s", got, want)
	}

	if _, err := calculateReminderFireAt(reminderDefinition{
		ScheduleKind: "before_date", DaysBefore: &days, TimeLocal: &local,
	}, reminderSource{Active: true, Moment: &anchor}); !errors.Is(err, ErrReminderScheduleMismatch) {
		t.Fatalf("shape mismatch error = %v", err)
	}
	if _, err := calculateReminderFireAt(reminderDefinition{
		ScheduleKind: "before_moment", MinutesBefore: &minutes,
	}, reminderSource{Active: false, Moment: &anchor}); !errors.Is(err, ErrReminderSourceUnscheduled) {
		t.Fatalf("inactive error = %v", err)
	}
}

func TestCalculateReminderFireAtRejectsDSTGapAndFold(t *testing.T) {
	days := 0
	for _, test := range []struct {
		name      string
		date      time.Time
		timeLocal string
	}{
		{name: "gap", date: time.Date(2026, time.March, 8, 0, 0, 0, 0, time.UTC), timeLocal: "02:30"},
		{name: "fold", date: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC), timeLocal: "01:30"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := calculateReminderFireAt(reminderDefinition{
				ScheduleKind: "before_date", DaysBefore: &days, TimeLocal: &test.timeLocal,
			}, reminderSource{Active: true, Date: &test.date, Timezone: "America/New_York"})
			if !errors.Is(err, ErrReminderInvalidLocalTime) {
				t.Fatalf("error = %v, want invalid local time", err)
			}
		})
	}
}
