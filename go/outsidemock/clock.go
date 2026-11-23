package outsidemock

import (
	"sync"
	"time"
)

// Clock は mock の時計。決めた時刻から始まり Advance と Sleep で進む。
type Clock struct {
	recorder
	mu  sync.Mutex
	now time.Time
}

// NewClock は start から始まる時計を返す。
func NewClock(start time.Time) *Clock { return &Clock{now: start} }

func (c *Clock) Now() time.Time {
	c.record("Now")
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Sleep は 実時間を待たず 時刻を d だけ進める。0 以下は 進めない。
func (c *Clock) Sleep(d time.Duration) {
	c.record("Sleep", d.String())
	c.advance(d)
}

// Advance は 時刻を d だけ進める。0 以下は 進めない。
func (c *Clock) Advance(d time.Duration) { c.advance(d) }

func (c *Clock) advance(d time.Duration) {
	if d <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
