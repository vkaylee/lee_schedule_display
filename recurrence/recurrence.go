// Package recurrence expands local-time schedules without storage or transport dependencies.
package recurrence

import (
	"errors"
	"fmt"
	"time"
)

const (
	MaxInterval    = 366
	MaxOccurrences = 10000
	MaxDuration    = 7 * 24 * time.Hour
	// MaxDays bounds the query window, independently of distance from Start.
	MaxDays = 366 * 500
)

var (
	ErrInvalid   = errors.New("invalid recurrence")
	ErrLimit     = errors.New("recurrence resource limit exceeded")
	ErrLocalTime = errors.New("local time is nonexistent or ambiguous")
	ErrUnbounded = errors.New("unbounded recurrence cannot be validated for storage")
)

type Frequency uint8

const (
	Daily Frequency = iota + 1
	Weekly
	Monthly
)

// Rule uses Start's wall clock in Location. Until is an inclusive local date;
// Count is the number of generated occurrences, including those outside a query.
// A zero Until and Count represent an unbounded series. At most one may be set.
// Monthly rules use either MonthDay or NthWeekday (1..5 or -1 for last).
type Rule struct {
	Start      time.Time
	Location   *time.Location
	Frequency  Frequency
	Interval   int
	Weekdays   []time.Weekday
	MonthDay   int
	Nth        int
	NthWeekday time.Weekday
	Until      time.Time
	Count      int
	Duration   time.Duration
}

type Occurrence struct {
	Start time.Time
	End   time.Time
}

// ValidateForStorage checks every scheduled local start in a finite rule against
// the currently installed timezone data. Unbounded rules cannot be certified.
// A future timezone database change requires validation again.
func ValidateForStorage(r Rule) error {
	if err := validate(r, r.Start, r.Start.Add(time.Second)); err != nil {
		return err
	}
	if r.Count == 0 && r.Until.IsZero() {
		return ErrUnbounded
	}
	local := r.Start.In(r.Location)
	first := civil(local)
	last := time.Date(2400, 12, 31, 0, 0, 0, 0, time.UTC)
	if !r.Until.IsZero() {
		last = civil(r.Until)
		if last.Year() > 2400 {
			return ErrLimit
		}
	}
	count := 0
	for date := first; !date.After(last); date = date.AddDate(0, 0, 1) {
		if !matches(r, first, date) {
			continue
		}
		count++
		if count > MaxOccurrences {
			return ErrLimit
		}
		if _, err := resolve(date, local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), r.Location); err != nil {
			return fmt.Errorf("%w: %s", err, date.Format("2006-01-02"))
		}
		if r.Count > 0 && count == r.Count {
			return nil
		}
	}
	if r.Count > 0 && count < r.Count {
		return ErrLimit
	}
	return nil
}

// Expand returns occurrences intersecting the half-open interval [from, to).
// It is not storage validation: callers must first use ValidateForStorage.
// A gap or fold fails the query only if its possible instants intersect it.
// No partial result is returned on error.
func Expand(r Rule, from, to time.Time) ([]Occurrence, error) {
	if err := validate(r, from, to); err != nil {
		return nil, err
	}
	local := r.Start.In(r.Location)
	startDate := civil(local)
	// Time zone offsets are constrained to 24 hours in validate. Scan a
	// conservative civil-date window to account for transitions and duration.
	firstDate := civil(from.UTC().Add(-r.Duration)).AddDate(0, 0, -1)
	lastDate := civil(to.UTC()).AddDate(0, 0, 1)
	if firstDate.Before(startDate) {
		firstDate = startDate
	}
	maxDate := time.Date(2400, 12, 31, 0, 0, 0, 0, time.UTC)
	if lastDate.After(maxDate) {
		lastDate = maxDate
	}
	if !r.Until.IsZero() && civil(r.Until).Before(lastDate) {
		lastDate = civil(r.Until)
	}
	if lastDate.Before(firstDate) {
		return []Occurrence{}, nil
	}
	if (lastDate.Unix()-firstDate.Unix())/86400 > MaxDays {
		return nil, ErrLimit
	}
	result := make([]Occurrence, 0)
	count := countBefore(r, startDate, firstDate)
	for date := firstDate; !date.After(lastDate); date = date.AddDate(0, 0, 1) {
		if !matches(r, startDate, date) {
			continue
		}
		count++
		if r.Count > 0 && count > r.Count {
			break
		}
		start, err := resolve(date, local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), r.Location)
		if err != nil {
			if mayIntersect(date, local, r.Location, r.Duration, from, to) {
				return nil, fmt.Errorf("%w: %s", err, date.Format("2006-01-02"))
			}
			continue
		}
		if start.Before(r.Start) {
			continue
		}
		end := start.Add(r.Duration)
		if start.Before(to) && end.After(from) {
			if len(result) == MaxOccurrences {
				return nil, ErrLimit
			}
			result = append(result, Occurrence{Start: start, End: end})
		}
	}
	return result, nil
}

func validate(r Rule, from, to time.Time) error {
	if r.Location == nil || r.Start.IsZero() || !from.Before(to) || r.Interval < 1 || r.Interval > MaxInterval || r.Duration <= 0 || r.Duration > MaxDuration || r.Count < 0 || r.Count > MaxOccurrences || (r.Count > 0 && !r.Until.IsZero()) {
		return ErrInvalid
	}
	local := r.Start.In(r.Location)
	for _, t := range []time.Time{r.Start, from, to} {
		_, offset := t.In(r.Location).Zone()
		if offset <= -86400 || offset >= 86400 {
			return ErrLimit
		}
	}
	if local.Year() < 1900 || local.Year() > 2400 {
		return ErrLimit
	}
	// Query bounds include every supported start plus the maximum duration,
	// with an extra day on each side for offsets strictly below 24 hours.
	minQuery := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Add(-24 * time.Hour)
	maxQuery := time.Date(2401, 1, 1, 0, 0, 0, 0, time.UTC).Add(24*time.Hour + MaxDuration)
	if from.Before(minQuery.Add(-MaxDuration)) || to.After(maxQuery) {
		return ErrLimit
	}
	resolved, err := resolve(civil(local), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), r.Location)
	if err != nil || !resolved.Equal(r.Start) {
		return ErrLocalTime
	}
	if !r.Until.IsZero() && civil(r.Until).Before(civil(local)) {
		return ErrInvalid
	}
	switch r.Frequency {
	case Daily:
		if len(r.Weekdays) != 0 || r.MonthDay != 0 || r.Nth != 0 {
			return ErrInvalid
		}
	case Weekly:
		if len(r.Weekdays) == 0 || len(r.Weekdays) > 7 || r.MonthDay != 0 || r.Nth != 0 {
			return ErrInvalid
		}
		seen := [7]bool{}
		for _, day := range r.Weekdays {
			if day < time.Sunday || day > time.Saturday || seen[day] {
				return ErrInvalid
			}
			seen[day] = true
		}
	case Monthly:
		if len(r.Weekdays) != 0 || !((r.MonthDay >= 1 && r.MonthDay <= 31 && r.Nth == 0) || (r.MonthDay == 0 && (r.Nth >= 1 && r.Nth <= 5 || r.Nth == -1) && r.NthWeekday >= time.Sunday && r.NthWeekday <= time.Saturday)) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if !matches(r, civil(local), civil(local)) {
		return ErrInvalid
	}
	return nil
}

// countBefore counts scheduled civil dates, including gap/fold dates, without
// resolving or traversing past occurrences. Gregorian monthly patterns repeat
// every 4800 months.
func countBefore(r Rule, start, date time.Time) int {
	days := int((date.Unix() - start.Unix()) / 86400)
	if days <= 0 {
		return 0
	}
	switch r.Frequency {
	case Daily:
		return (days-1)/r.Interval + 1
	case Weekly:
		count := 0
		for _, weekday := range r.Weekdays {
			delta := (int(weekday) - int(start.Weekday()) + 7) % 7
			// Week zero is anchored to Monday, not to Start.
			if (int(start.Weekday())+6)%7+delta >= 7 {
				delta += 7 * (r.Interval - 1)
			}
			if delta < days {
				count += 1 + (days-1-delta)/(7*r.Interval)
			}
		}
		return count
	case Monthly:
		months := (date.Year()-start.Year())*12 + int(date.Month()-start.Month())
		steps := months/r.Interval + 1
		cycle := 4800 / gcd(4800, r.Interval)
		count := 0
		for i := 0; i < cycle && i < steps; i++ {
			month := time.Date(start.Year(), start.Month()+time.Month(i*r.Interval), 1, 0, 0, 0, 0, time.UTC)
			candidate := monthlyDate(r, month)
			if !candidate.IsZero() {
				repeats := 1 + (steps-1-i)/cycle
				count += repeats
				if i == 0 && candidate.Before(start) {
					count--
				}
				if (steps-1-i)%cycle == 0 && !candidate.AddDate(400*((steps-1-i)/cycle)*(r.Interval/gcd(4800, r.Interval)), 0, 0).Before(date) {
					count--
				}
			}
		}
		return count
	}
	return 0
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func monthlyDate(r Rule, month time.Time) time.Time {
	if r.MonthDay != 0 {
		d := month.AddDate(0, 0, r.MonthDay-1)
		if d.Month() == month.Month() {
			return d
		}
		return time.Time{}
	}
	if r.Nth == -1 {
		last := month.AddDate(0, 1, -1)
		return last.AddDate(0, 0, -(int(last.Weekday())-int(r.NthWeekday)+7)%7)
	}
	d := month.AddDate(0, 0, (int(r.NthWeekday)-int(month.Weekday())+7)%7+7*(r.Nth-1))
	if d.Month() != month.Month() {
		return time.Time{}
	}
	return d
}

// mayIntersect checks both sides of nearby transitions for gap/fold instants.
func mayIntersect(date, local time.Time, loc *time.Location, duration time.Duration, from, to time.Time) bool {
	wall := time.Date(date.Year(), date.Month(), date.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
	for cursor, end := wall.Add(-72*time.Hour), wall.Add(72*time.Hour); cursor.Before(end); {
		z := cursor.In(loc)
		_, offset := z.Zone()
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		if candidate.Before(to) && candidate.Add(duration).After(from) {
			return true
		}
		_, next := z.ZoneBounds()
		if next.IsZero() || !next.After(cursor) {
			break
		}
		cursor = next
	}
	return false
}

func civil(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func matches(r Rule, start, date time.Time) bool {
	days := int((date.Unix() - start.Unix()) / 86400)
	switch r.Frequency {
	case Daily:
		return days%r.Interval == 0
	case Weekly:
		startMonday := (int(start.Weekday()) + 6) % 7
		if (days+startMonday)/7%r.Interval != 0 {
			return false
		}
		for _, day := range r.Weekdays {
			if date.Weekday() == day {
				return true
			}
		}
	case Monthly:
		months := (date.Year()-start.Year())*12 + int(date.Month()-start.Month())
		if months%r.Interval != 0 {
			return false
		}
		if r.MonthDay != 0 {
			return date.Day() == r.MonthDay
		}
		if date.Weekday() != r.NthWeekday {
			return false
		}
		if r.Nth == -1 {
			return date.AddDate(0, 0, 7).Month() != date.Month()
		}
		return (date.Day()-1)/7+1 == r.Nth
	}
	return false
}

// resolve enumerates offsets in the surrounding timezone intervals and accepts
// exactly one matching instant; time.Date alone silently normalizes gaps/folds.
func resolve(date time.Time, hour, minute, second, nanos int, loc *time.Location) (time.Time, error) {
	wall := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, second, nanos, time.UTC)
	begin, end := wall.Add(-72*time.Hour), wall.Add(72*time.Hour)
	seen := map[int]bool{}
	var found time.Time
	for cursor := begin; cursor.Before(end); {
		zoned := cursor.In(loc)
		_, offset := zoned.Zone()
		if !seen[offset] {
			seen[offset] = true
			candidate := wall.Add(-time.Duration(offset) * time.Second)
			if candidate.In(loc).Format("2006-01-02T15:04:05.999999999") == wall.Format("2006-01-02T15:04:05.999999999") {
				if !found.IsZero() {
					return time.Time{}, ErrLocalTime
				}
				found = candidate
			}
		}
		_, next := zoned.ZoneBounds()
		if next.IsZero() || !next.After(cursor) {
			break
		}
		cursor = next
	}
	if found.IsZero() {
		return time.Time{}, ErrLocalTime
	}
	return found, nil
}
