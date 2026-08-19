package timeutil

import (
	_ "time/tzdata"

	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidLocalTime   = errors.New("invalid local time")
	ErrNonexistentTime    = errors.New("nonexistent local time")
	ErrAmbiguousLocalTime = errors.New("ambiguous local time")
)

func LoadLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "Local" || len(name) > 100 || strings.Contains(name, "..") || strings.ContainsRune(name, '\x00') {
		return nil, fmt.Errorf("invalid timezone")
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}
	return location, nil
}

// ParseLocalWallTime resolves a timezone-free wall-clock value to exactly one
// instant. It rejects both DST gaps and folds so the API never silently moves
// or guesses a user's due time.
func ParseLocalWallTime(value string, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	var wall time.Time
	var parseErr error
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05"} {
		wall, parseErr = time.Parse(layout, value)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return time.Time{}, ErrInvalidLocalTime
	}

	offsets := make(map[int]struct{})
	for candidate := wall.Add(-36 * time.Hour); !candidate.After(wall.Add(36 * time.Hour)); candidate = candidate.Add(15 * time.Minute) {
		_, offset := candidate.In(location).Zone()
		offsets[offset] = struct{}{}
	}

	matches := make([]time.Time, 0, 2)
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(location)
		if sameWallClock(wall, local) {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 0:
		return time.Time{}, ErrNonexistentTime
	case 1:
		return matches[0].UTC(), nil
	default:
		return time.Time{}, ErrAmbiguousLocalTime
	}
}

func LocalDayWindow(now time.Time, location *time.Location) (date string, start, end time.Time) {
	local := now.In(location)
	startLocal := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	endLocal := startLocal.AddDate(0, 0, 1)
	return startLocal.Format(time.DateOnly), startLocal.UTC(), endLocal.UTC()
}

func sameWallClock(wall, local time.Time) bool {
	return wall.Year() == local.Year() &&
		wall.Month() == local.Month() &&
		wall.Day() == local.Day() &&
		wall.Hour() == local.Hour() &&
		wall.Minute() == local.Minute() &&
		wall.Second() == local.Second()
}
