package unbounded

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCapTrackerExceedsAndRollsOver(t *testing.T) {
	now := time.Date(2026, 10, 29, 15, 0, 0, 0, time.Local)
	clock := func() time.Time { return now }
	dir := t.TempDir()
	c, err := newCapTracker(DataCap{Bytes: 1000, Period: PeriodDay}, dir, clock)
	if err != nil {
		t.Fatal(err)
	}
	c.add(600)
	select {
	case <-c.Exceeded():
		t.Fatal("exceeded before the cap")
	default:
	}
	c.add(400)
	select {
	case <-c.Exceeded():
	default:
		t.Fatal("not exceeded at the cap")
	}
	if got, want := c.NextReset(), time.Date(2026, 10, 30, 0, 0, 0, 0, time.Local); !got.Equal(want) {
		t.Fatalf("NextReset = %v, want %v", got, want)
	}
	if c.Rollover() {
		t.Fatal("rolled over before the period ended")
	}

	// A restart in the same period keeps the count.
	if err := c.persist(); err != nil {
		t.Fatal(err)
	}
	again, err := newCapTracker(DataCap{Bytes: 1000, Period: PeriodDay}, dir, clock)
	if err != nil {
		t.Fatal(err)
	}
	if again.Used() != 1000 {
		t.Fatalf("restored %d bytes, want 1000", again.Used())
	}
	select {
	case <-again.Exceeded():
	default:
		t.Fatal("restored tracker should start exceeded")
	}

	// The next day starts fresh, and a restart then ignores the stale file.
	now = now.Add(24 * time.Hour)
	if !c.Rollover() {
		t.Fatal("did not roll over after the period ended")
	}
	if c.Used() != 0 {
		t.Fatalf("used %d after rollover", c.Used())
	}
	select {
	case <-c.Exceeded():
		t.Fatal("exceeded right after rollover")
	default:
	}
	fresh, err := newCapTracker(DataCap{Bytes: 1000, Period: PeriodDay}, dir, clock)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Used() != 0 {
		t.Fatalf("stale usage file restored into a new period: %d", fresh.Used())
	}
	if _, err := filepath.Abs(dir); err != nil {
		t.Fatal(err)
	}
}

func TestPeriodStart(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	wed := time.Date(2026, 10, 28, 20, 27, 0, 0, loc)
	cases := []struct {
		p    Period
		want time.Time
		next time.Time
	}{
		{PeriodDay, time.Date(2026, 10, 28, 0, 0, 0, 0, loc), time.Date(2026, 10, 29, 0, 0, 0, 0, loc)},
		{PeriodWeek, time.Date(2026, 10, 26, 0, 0, 0, 0, loc), time.Date(2026, 11, 2, 0, 0, 0, 0, loc)},
		{PeriodMonth, time.Date(2026, 10, 1, 0, 0, 0, 0, loc), time.Date(2026, 11, 1, 0, 0, 0, 0, loc)},
	}
	for _, c := range cases {
		got := periodStart(c.p, wed)
		if !got.Equal(c.want) {
			t.Errorf("periodStart(%v) = %v, want %v", c.p, got, c.want)
		}
		if next := nextPeriodStart(c.p, got); !next.Equal(c.next) {
			t.Errorf("nextPeriodStart(%v) = %v, want %v", c.p, next, c.next)
		}
	}
}
