package recurrence

import (
	"errors"
	"testing"
	"time"
)

func instant(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
func base() Rule {
	return Rule{Start: instant("2024-01-31T10:00:00Z"), Location: time.UTC, Frequency: Monthly, Interval: 1, MonthDay: 31, Duration: time.Hour}
}
func TestMonth31AndOverlap(t *testing.T) {
	r := base()
	r.Count = 3
	got, err := Expand(r, instant("2024-02-01T00:00:00Z"), instant("2024-06-01T00:00:00Z"))
	if err != nil || len(got) != 2 || got[0].Start != instant("2024-03-31T10:00:00Z") || got[1].Start != instant("2024-05-31T10:00:00Z") {
		t.Fatalf("%v %v", got, err)
	}
	got, err = Expand(r, instant("2024-01-31T10:30:00Z"), instant("2024-01-31T11:00:00Z"))
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	got, err = Expand(r, instant("2024-01-31T11:00:00Z"), instant("2024-02-01T00:00:00Z"))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}
func TestWeeklyAndFarQuery(t *testing.T) {
	r := Rule{Start: instant("2024-01-01T09:00:00Z"), Location: time.UTC, Frequency: Weekly, Interval: 2, Weekdays: []time.Weekday{time.Monday, time.Wednesday}, Duration: time.Hour}
	got, err := Expand(r, instant("2024-01-01T00:00:00Z"), instant("2024-01-20T00:00:00Z"))
	if err != nil || len(got) != 4 {
		t.Fatalf("%v %v", got, err)
	}
	got, err = Expand(r, instant("2200-01-01T00:00:00Z"), instant("2200-02-01T00:00:00Z"))
	if err != nil || len(got) == 0 {
		t.Fatalf("%v %v", got, err)
	}
}
func TestDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, start := range []string{"2024-03-09T07:30:00Z", "2024-11-02T05:30:00Z"} {
		r := Rule{Start: instant(start), Location: loc, Frequency: Daily, Interval: 1, Duration: time.Hour}
		_, err := Expand(r, instant(start), instant(start).Add(72*time.Hour))
		if !errors.Is(err, ErrLocalTime) {
			t.Fatalf("%s: %v", start, err)
		}
	}
}
func TestValidateForStorage(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, start := range []string{"2024-03-09T07:30:00Z", "2024-11-02T05:30:00Z"} {
		r := Rule{Start: instant(start), Location: loc, Frequency: Daily, Interval: 1, Duration: time.Hour, Count: 4}
		if err := ValidateForStorage(r); !errors.Is(err, ErrLocalTime) {
			t.Fatalf("count %s: %v", start, err)
		}
		r.Count = 0
		r.Until = civil(r.Start.In(loc)).AddDate(0, 0, 3)
		if err := ValidateForStorage(r); !errors.Is(err, ErrLocalTime) {
			t.Fatalf("until %s: %v", start, err)
		}
	}
	r := base()
	r.Count = 3
	if err := ValidateForStorage(r); err != nil {
		t.Fatal(err)
	}
	r.Count = 0
	if err := ValidateForStorage(r); !errors.Is(err, ErrUnbounded) {
		t.Fatal(err)
	}
}

func TestSupportedDateBoundaries(t *testing.T) {
	for _, offset := range []int{0, 14 * 3600, -12 * 3600} {
		loc := time.FixedZone("boundary", offset)
		for _, local := range []time.Time{time.Date(2400, 12, 31, 23, 59, 59, 0, loc), time.Date(1900, 1, 1, 0, 0, 0, 0, loc)} {
			r := Rule{Start: local, Location: loc, Frequency: Daily, Interval: 1, Count: 1, Duration: time.Hour}
			if err := ValidateForStorage(r); err != nil {
				t.Fatalf("%v: %v", local, err)
			}
			got, err := Expand(r, local.Add(-time.Minute), local.Add(time.Hour))
			if err != nil || len(got) != 1 || !got[0].Start.Equal(local) {
				t.Fatalf("%v: %v %v", local, got, err)
			}
		}
	}
	r := Rule{Start: instant("2401-01-01T00:00:00Z"), Location: time.UTC, Frequency: Daily, Interval: 1, Count: 1, Duration: time.Hour}
	if err := ValidateForStorage(r); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}

func TestDSTOutsideQueryAndCount(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	r := Rule{Start: instant("2024-03-09T07:30:00Z"), Location: loc, Frequency: Daily, Interval: 1, Duration: time.Hour, Count: 4}
	got, err := Expand(r, instant("2024-03-11T00:00:00Z"), instant("2024-03-14T00:00:00Z"))
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	got, err = Expand(r, instant("2024-03-09T00:00:00Z"), instant("2024-03-10T00:00:00Z"))
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestFarFiniteCount(t *testing.T) {
	r := base()
	r.Frequency = Daily
	r.MonthDay = 0
	r.Count = 2
	got, err := Expand(r, instant("2399-01-01T00:00:00Z"), instant("2400-01-01T00:00:00Z"))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestCountBeforeMatchesBruteForce(t *testing.T) {
	start := civil(instant("2024-01-31T10:00:00Z"))
	rules := []Rule{
		{Frequency: Daily, Interval: 7},
		{Frequency: Weekly, Interval: 3, Weekdays: []time.Weekday{time.Monday, time.Wednesday, time.Sunday}},
		{Frequency: Monthly, Interval: 1, MonthDay: 31},
		{Frequency: Monthly, Interval: 7, Nth: 5, NthWeekday: time.Wednesday},
		{Frequency: Monthly, Interval: 12, Nth: -1, NthWeekday: time.Monday},
	}
	for _, r := range rules {
		count := 0
		for d := start; d.Before(start.AddDate(20, 0, 0)); d = d.AddDate(0, 0, 1) {
			if got := countBefore(r, start, d); got != count {
				t.Fatalf("%+v %v: got %d want %d", r, d, got, count)
			}
			if matches(r, start, d) {
				count++
			}
		}
	}
}

func TestMonthlyCountAcrossGregorianCycle(t *testing.T) {
	start := civil(instant("1900-01-31T10:00:00Z"))
	for _, interval := range []int{1, 7, 12, 366} {
		r := Rule{Frequency: Monthly, Interval: interval, MonthDay: 31}
		for year := 1900; year <= 2400; year++ {
			end := time.Date(year, 6, 1, 0, 0, 0, 0, time.UTC)
			want := 0
			for m := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC); m.Before(end); m = m.AddDate(0, interval, 0) {
				d := monthlyDate(r, m)
				if !d.IsZero() && !d.Before(start) && d.Before(end) {
					want++
				}
			}
			if got := countBefore(r, start, end); got != want {
				t.Fatalf("interval %d year %d: %d != %d", interval, year, got, want)
			}
		}
	}
}

func TestValidationAndLimit(t *testing.T) {
	r := base()
	r.Interval = 0
	if _, err := Expand(r, r.Start, r.Start.Add(time.Hour)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	r = base()
	if _, err := Expand(r, instant("2399-01-01T00:00:00Z"), instant("2401-02-01T00:00:00Z")); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	r = base()
	r.Frequency = Daily
	r.MonthDay = 0
	if _, err := Expand(r, r.Start, instant("2100-01-01T00:00:00Z")); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}
