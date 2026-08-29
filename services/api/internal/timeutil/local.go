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
	ErrSkippedLocalDate   = errors.New("local calendar date does not exist")
)

const localDateSearchMargin = 48 * time.Hour

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

// ResolveRecurringLocalWallTime gives a recurrence series deterministic DST
// semantics. A fold chooses the earlier instant. A gap shifts the requested
// wall clock forward by the exact offset jump (02:30 → 03:30 for a one-hour
// spring transition) instead of rejecting or drifting the series.
func ResolveRecurringLocalWallTime(value string, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	var wall time.Time
	var parseErr error
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05"} {
		wall, parseErr = time.Parse(layout, value)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil || location == nil {
		return time.Time{}, ErrInvalidLocalTime
	}

	offsets := make(map[int]struct{})
	for cursor := wall.Add(-36 * time.Hour); !cursor.After(wall.Add(36 * time.Hour)); cursor = cursor.Add(15 * time.Minute) {
		_, offset := cursor.In(location).Zone()
		offsets[offset] = struct{}{}
	}
	matches := make([]time.Time, 0, 2)
	var shifted time.Time
	var smallestPositiveShift time.Duration
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(location)
		if sameWallClock(wall, local) {
			matches = append(matches, candidate.UTC())
			continue
		}
		localWall := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
		shift := localWall.Sub(wall)
		if shift > 0 && (shifted.IsZero() || shift < smallestPositiveShift || (shift == smallestPositiveShift && candidate.Before(shifted))) {
			shifted = candidate.UTC()
			smallestPositiveShift = shift
		}
	}
	if len(matches) > 0 {
		earliest := matches[0]
		for _, candidate := range matches[1:] {
			if candidate.Before(earliest) {
				earliest = candidate
			}
		}
		return earliest, nil
	}
	if !shifted.IsZero() {
		return shifted, nil
	}
	return time.Time{}, ErrNonexistentTime
}

// LocalDateStart returns the earliest instant whose civil date in location is
// the supplied year, month, and day. It does not ask time.Date to guess through
// a midnight transition: each fixed-offset zone interval is intersected with
// the requested wall-clock day instead. A date skipped by an offset change,
// such as Pacific/Apia 2011-12-30, returns ErrSkippedLocalDate.
func LocalDateStart(date time.Time, location *time.Location) (time.Time, error) {
	if location == nil {
		return time.Time{}, errors.New("local date location is nil")
	}

	wallStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	wallEnd := wallStart.AddDate(0, 0, 1)
	searchStart := wallStart.Add(-localDateSearchMargin)
	searchEnd := wallEnd.Add(localDateSearchMargin)

	var earliest time.Time
	found := false
	for cursor := searchStart; cursor.Before(searchEnd); {
		localCursor := cursor.In(location)
		_, offset := localCursor.Zone()
		zoneStart, zoneEnd := localCursor.ZoneBounds()

		intervalStart := searchStart
		if !zoneStart.IsZero() && zoneStart.After(intervalStart) {
			intervalStart = zoneStart
		}
		intervalEnd := searchEnd
		if !zoneEnd.IsZero() && zoneEnd.Before(intervalEnd) {
			intervalEnd = zoneEnd
		}

		candidateStart := wallStart.Add(-time.Duration(offset) * time.Second)
		candidateEnd := wallEnd.Add(-time.Duration(offset) * time.Second)
		if candidateStart.Before(intervalStart) {
			candidateStart = intervalStart
		}
		if candidateEnd.After(intervalEnd) {
			candidateEnd = intervalEnd
		}
		if candidateStart.Before(candidateEnd) && (!found || candidateStart.Before(earliest)) {
			candidateLocal := candidateStart.In(location)
			if candidateLocal.Year() == date.Year() && candidateLocal.Month() == date.Month() && candidateLocal.Day() == date.Day() {
				earliest = candidateStart
				found = true
			}
		}

		if zoneEnd.IsZero() || !zoneEnd.Before(searchEnd) {
			break
		}
		if !zoneEnd.After(cursor) {
			return time.Time{}, errors.New("invalid local timezone bounds")
		}
		cursor = zoneEnd
	}

	if !found {
		return time.Time{}, ErrSkippedLocalDate
	}
	return earliest.UTC(), nil
}

// LocalDateEnd returns the first representable local-date boundary after date.
// It validates date itself, then passes over any wholly skipped following date
// so an inclusive civil-date range remains a correct half-open instant range.
func LocalDateEnd(date time.Time, location *time.Location) (time.Time, error) {
	if _, err := LocalDateStart(date, location); err != nil {
		return time.Time{}, err
	}
	for next := date.AddDate(0, 0, 1); ; next = next.AddDate(0, 0, 1) {
		start, err := LocalDateStart(next, location)
		if err == nil {
			return start, nil
		}
		if !errors.Is(err, ErrSkippedLocalDate) {
			return time.Time{}, err
		}
	}
}

func LocalDayWindow(now time.Time, location *time.Location) (date string, start, end time.Time, err error) {
	local := now.In(location)
	localDate := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	start, err = LocalDateStart(localDate, location)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	end, err = LocalDateEnd(localDate, location)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	return localDate.Format(time.DateOnly), start, end, nil
}

func sameWallClock(wall, local time.Time) bool {
	return wall.Year() == local.Year() &&
		wall.Month() == local.Month() &&
		wall.Day() == local.Day() &&
		wall.Hour() == local.Hour() &&
		wall.Minute() == local.Minute() &&
		wall.Second() == local.Second()
}
