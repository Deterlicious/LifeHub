package recurrence

import (
	"errors"
	"time"
)

var ErrInvalidRule = errors.New("invalid recurrence rule")

const (
	FrequencyDaily   = "daily"
	FrequencyWeekly  = "weekly"
	FrequencyMonthly = "monthly"
	FrequencyYearly  = "yearly"
	MaxInterval      = 365
)

type Rule struct {
	Frequency string
	Interval  int
	EndsOn    *time.Time
}

func (rule Rule) Validate(anchor time.Time) error {
	if rule.Interval < 1 || rule.Interval > MaxInterval {
		return ErrInvalidRule
	}
	switch rule.Frequency {
	case FrequencyDaily, FrequencyWeekly, FrequencyMonthly, FrequencyYearly:
	default:
		return ErrInvalidRule
	}
	anchor = dateOnly(anchor)
	if rule.EndsOn != nil && dateOnly(*rule.EndsOn).Before(anchor) {
		return ErrInvalidRule
	}
	return nil
}

// Dates returns concrete occurrence dates in [from, through]. The anchor is
// always returned once when it is not beyond ends_on, even if it is older than
// from. This preserves the user's explicit first occurrence without backfilling
// every historical recurrence between an old anchor and Today.
func Dates(anchor, from, through time.Time, rule Rule) ([]time.Time, error) {
	anchor = dateOnly(anchor)
	from = dateOnly(from)
	through = dateOnly(through)
	if err := rule.Validate(anchor); err != nil {
		return nil, err
	}
	if through.Before(from) {
		return nil, ErrInvalidRule
	}
	endsOn := through
	if rule.EndsOn != nil && dateOnly(*rule.EndsOn).Before(endsOn) {
		endsOn = dateOnly(*rule.EndsOn)
	}
	if endsOn.Before(anchor) {
		return []time.Time{anchor}, nil
	}

	result := make([]time.Time, 0, 16)
	result = append(result, anchor)
	startIndex := firstIndexOnOrAfter(anchor, from, rule.Frequency, rule.Interval)
	for index := startIndex; ; index++ {
		candidate := occurrenceAt(anchor, rule.Frequency, rule.Interval, index)
		if candidate.After(endsOn) {
			break
		}
		if !candidate.Before(from) {
			result = append(result, candidate)
		}
	}
	return result, nil
}

func firstIndexOnOrAfter(anchor, from time.Time, frequency string, interval int) int {
	if !from.After(anchor) {
		return 1
	}
	var estimate int
	switch frequency {
	case FrequencyDaily:
		days := int(from.Sub(anchor) / (24 * time.Hour))
		estimate = days / interval
	case FrequencyWeekly:
		days := int(from.Sub(anchor) / (24 * time.Hour))
		estimate = days / (7 * interval)
	case FrequencyMonthly:
		months := (from.Year()-anchor.Year())*12 + int(from.Month()-anchor.Month())
		estimate = months / interval
	case FrequencyYearly:
		estimate = (from.Year() - anchor.Year()) / interval
	}
	if estimate < 1 {
		estimate = 1
	}
	for estimate > 1 && !occurrenceAt(anchor, frequency, interval, estimate-1).Before(from) {
		estimate--
	}
	for occurrenceAt(anchor, frequency, interval, estimate).Before(from) {
		estimate++
	}
	return estimate
}

func occurrenceAt(anchor time.Time, frequency string, interval, index int) time.Time {
	step := interval * index
	switch frequency {
	case FrequencyDaily:
		return anchor.AddDate(0, 0, step)
	case FrequencyWeekly:
		return anchor.AddDate(0, 0, 7*step)
	case FrequencyMonthly:
		return clampedDate(anchor, 0, step)
	case FrequencyYearly:
		return clampedDate(anchor, step, 0)
	default:
		panic("validated recurrence frequency")
	}
}

// clampedDate always uses the original anchor day. It therefore produces
// Jan31 → Feb28/29 → Mar31 instead of drifting through time.AddDate overflow.
func clampedDate(anchor time.Time, years, months int) time.Time {
	monthIndex := int(anchor.Month()) - 1 + months
	year := anchor.Year() + years + monthIndex/12
	monthIndex %= 12
	if monthIndex < 0 {
		monthIndex += 12
		year--
	}
	month := time.Month(monthIndex + 1)
	day := anchor.Day()
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
