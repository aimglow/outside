package outsidemock

import (
	"sync"
	"time"

	"github.com/aimglow/outside/go"
)

// Clock は mock の時計。決めた時刻から始まり Advance と Sleep で進む。
// Advance・Sleep は 期限を過ぎた Timer・Ticker・After を 期限の順に発火する (channel に その期限の時刻を送る)。
// 期限が同じ物は 掛けた順。channel は 1 つ分の buffer を持ち 満ちていれば その発火は落ちる (time と同じ)。
// 並行に呼んでも安全。実時間は待たない。
type Clock struct {
	recorder
	mu      sync.Mutex
	now     time.Time
	waiters []*waiter
	seq     int
}

// waiter は 掛かっている Timer か Ticker 1 つ。period が 0 なら Timer。
type waiter struct {
	when   time.Time
	period time.Duration
	ch     chan time.Time
	active bool
	seq    int
}

// NewClock は start から始まる時計を返す。
func NewClock(start time.Time) *Clock { return &Clock{now: start} }

func (c *Clock) Now() time.Time {
	c.record("Now")
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Sleep は 実時間を待たず 時刻を d だけ進める (期限を過ぎた物を発火する)。0 以下は 進めない。
func (c *Clock) Sleep(d time.Duration) {
	c.record("Sleep", d.String())
	c.advance(d)
}

// Advance は 時刻を d だけ進め 期限を過ぎた物を発火する。0 以下は 進めない。
func (c *Clock) Advance(d time.Duration) { c.advance(d) }

// NewTimer は 今から d 後に発火する Timer を返す。0 以下は 掛けた時に 今の時刻を送る。
func (c *Clock) NewTimer(d time.Duration) outside.Timer {
	c.record("NewTimer", d.String())
	return &mockTimer{c: c, w: c.start(d, 0)}
}

// After は NewTimer(d) の channel を返す。
func (c *Clock) After(d time.Duration) <-chan time.Time {
	c.record("After", d.String())
	return c.start(d, 0).ch
}

// NewTicker は 今から d ごとに発火する Ticker を返す。0 以下は panic (time.NewTicker と同じ)。
func (c *Clock) NewTicker(d time.Duration) outside.Ticker {
	c.record("NewTicker", d.String())
	if d <= 0 {
		panic("outsidemock: non-positive interval for NewTicker")
	}
	return &mockTicker{c: c, w: c.start(d, d)}
}

// start は 新しい waiter を掛ける。
func (c *Clock) start(d, period time.Duration) *waiter {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := &waiter{ch: make(chan time.Time, 1), period: period}
	c.arm(w, d)
	return w
}

// arm は w の期限を 今から d 後に掛ける。d が 0 以下の Timer は すぐ発火して 止まる。c.mu を持って呼ぶ。
func (c *Clock) arm(w *waiter, d time.Duration) {
	if d <= 0 && w.period == 0 {
		c.disarm(w)
		send(w.ch, c.now)
		return
	}
	w.when = c.now.Add(d)
	if !w.active {
		w.active = true
		c.seq++
		w.seq = c.seq
		c.waiters = append(c.waiters, w)
	}
}

// disarm は w を外し 届いていない値を捨てる (Stop・Reset の後に 前の期限の値は届かない = time と同じ)。
// 外す前に 動いていたかを返す。c.mu を持って呼ぶ。
func (c *Clock) disarm(w *waiter) bool {
	was := w.active
	if was {
		w.active = false
		for i, x := range c.waiters {
			if x == w {
				c.waiters = append(c.waiters[:i], c.waiters[i+1:]...)
				break
			}
		}
	}
	select {
	case <-w.ch:
	default:
	}
	return was
}

// send は 満ちていなければ t を送る。満ちていれば 落とす。送れたかを返す。
func send(ch chan time.Time, t time.Time) bool {
	select {
	case ch <- t:
		return true
	default:
		return false
	}
}

func (c *Clock) advance(d time.Duration) {
	if d <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	target := c.now.Add(d)
	for {
		w := c.next(target, nil)
		if w == nil {
			break
		}
		// 発火の間の Now は その期限 (時刻は戻らない)。
		c.now = w.when
		sent := send(w.ch, w.when)
		if w.period == 0 {
			c.disarm2(w)
			continue
		}
		w.when = w.when.Add(w.period)
		if !sent {
			// 満ちた Ticker は 次に別の物が発火するか target に着くまで 送っても落ちる ∴ その間の回は 数えて飛ばす。
			limit := target
			if o := c.next(target, w); o != nil && o.when.Before(limit) {
				limit = o.when
			}
			if !w.when.After(limit) {
				n := limit.Sub(w.when)/w.period + 1
				w.when = w.when.Add(n * w.period)
			}
		}
	}
	c.now = target
}

// next は target までに期限が来る waiter のうち 期限が最も早い物 (同じなら 掛けた順) を返す。skip は除く。
func (c *Clock) next(target time.Time, skip *waiter) *waiter {
	var best *waiter
	for _, w := range c.waiters {
		if w == skip || w.when.After(target) {
			continue
		}
		if best == nil || w.when.Before(best.when) || (w.when.Equal(best.when) && w.seq < best.seq) {
			best = w
		}
	}
	return best
}

// disarm2 は 発火した Timer を外す (送った値は 捨てない)。c.mu を持って呼ぶ。
func (c *Clock) disarm2(w *waiter) {
	w.active = false
	for i, x := range c.waiters {
		if x == w {
			c.waiters = append(c.waiters[:i], c.waiters[i+1:]...)
			return
		}
	}
}

type mockTimer struct {
	c *Clock
	w *waiter
}

func (t *mockTimer) C() <-chan time.Time { return t.w.ch }

func (t *mockTimer) Stop() bool {
	t.c.record("Timer.Stop")
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	return t.c.disarm(t.w)
}

func (t *mockTimer) Reset(d time.Duration) bool {
	t.c.record("Timer.Reset", d.String())
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	was := t.c.disarm(t.w)
	t.c.arm(t.w, d)
	return was
}

type mockTicker struct {
	c *Clock
	w *waiter
}

func (t *mockTicker) C() <-chan time.Time { return t.w.ch }

func (t *mockTicker) Stop() {
	t.c.record("Ticker.Stop")
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	t.c.disarm(t.w)
}
