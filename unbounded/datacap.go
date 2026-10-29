package unbounded

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// usageFileName is where the current period's relayed byte count lives, so a
// restart does not reset the cap.
const usageFileName = "unbounded-usage.json"

type usageFile struct {
	PeriodStart time.Time `json:"period_start"`
	Bytes       uint64    `json:"bytes"`
}

// capTracker counts relayed bytes against a DataCap and says when the budget
// for the current period is spent and when the next period begins. add runs
// on broflake's data path, so it is an atomic add and a compare; everything
// else takes the mutex.
type capTracker struct {
	cap  DataCap
	path string
	now  func() time.Time

	used atomic.Uint64

	mu          sync.Mutex
	periodStart time.Time
	exceeded    chan struct{}
	closed      bool
}

func newCapTracker(cap DataCap, dataPath string, now func() time.Time) (*capTracker, error) {
	if now == nil {
		now = time.Now
	}
	c := &capTracker{
		cap:      cap,
		now:      now,
		exceeded: make(chan struct{}),
	}
	if dataPath != "" {
		c.path = filepath.Join(dataPath, usageFileName)
	}
	c.periodStart = periodStart(cap.Period, now())
	if err := c.load(); err != nil {
		return nil, err
	}
	if c.used.Load() >= cap.Bytes {
		c.markExceeded()
	}
	return c, nil
}

// load restores the persisted count if it belongs to the current period.
func (c *capTracker) load() error {
	if c.path == "" {
		return nil
	}
	data, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read usage file: %w", err)
	}
	var f usageFile
	if err := json.Unmarshal(data, &f); err != nil {
		// A corrupt file is not worth refusing to start over; the period
		// just starts from zero.
		return nil
	}
	if f.PeriodStart.Equal(c.periodStart) {
		c.used.Store(f.Bytes)
	}
	return nil
}

// persist writes the current count. Called periodically and on stop.
func (c *capTracker) persist() error {
	if c.path == "" {
		return nil
	}
	c.mu.Lock()
	f := usageFile{PeriodStart: c.periodStart, Bytes: c.used.Load()}
	c.mu.Unlock()
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

// add charges n relayed bytes. Safe to call from any goroutine.
func (c *capTracker) add(n int) {
	if n <= 0 {
		return
	}
	total := c.used.Add(uint64(n))
	if total >= c.cap.Bytes {
		c.markExceeded()
	}
}

// markExceeded closes the current period's channel if its budget really is
// spent. An add that crossed the previous period's cap can be preempted by
// Rollover, which resets the count; the re-check under mu keeps it from
// closing the new period's channel.
func (c *capTracker) markExceeded() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.used.Load() < c.cap.Bytes {
		return
	}
	close(c.exceeded)
	c.closed = true
}

// Exceeded is closed once the current period's budget is spent.
func (c *capTracker) Exceeded() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.exceeded
}

// Used is the byte count so far this period.
func (c *capTracker) Used() uint64 {
	return c.used.Load()
}

// NextReset is when the current period ends.
func (c *capTracker) NextReset() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return nextPeriodStart(c.cap.Period, c.periodStart)
}

// Rollover starts a new period if the current one has ended and reports
// whether it did.
func (c *capTracker) Rollover() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if now.Before(nextPeriodStart(c.cap.Period, c.periodStart)) {
		return false
	}
	c.periodStart = periodStart(c.cap.Period, now)
	c.used.Store(0)
	c.exceeded = make(chan struct{})
	c.closed = false
	return true
}

// periodStart truncates t to the start of its period in t's location.
func periodStart(p Period, t time.Time) time.Time {
	y, m, d := t.Date()
	switch p {
	case PeriodWeek:
		// ISO weeks start on Monday.
		offset := (int(t.Weekday()) + 6) % 7
		return time.Date(y, m, d-offset, 0, 0, 0, 0, t.Location())
	case PeriodMonth:
		return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	}
}

// nextPeriodStart is the start of the period after the one beginning at start.
func nextPeriodStart(p Period, start time.Time) time.Time {
	switch p {
	case PeriodWeek:
		return start.AddDate(0, 0, 7)
	case PeriodMonth:
		return start.AddDate(0, 1, 0)
	default:
		return start.AddDate(0, 0, 1)
	}
}
