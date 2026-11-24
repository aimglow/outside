package good

import "time"

// Clock は outside の口と同じ形 (時刻は受け取る)。
type Clock interface{ Now() time.Time }

// Good は 口から時刻を受ける (lint が通すべき形)。
func Good(c Clock) time.Time { return c.Now() }
