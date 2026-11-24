package good

import (
	"time"

	"lintfixture/good/rand"
)

// Clock は outside の口と同じ形 (時刻は受け取る)。
type Clock interface{ Now() time.Time }

// SysInfo は outside の口と同じ形 (機体と process の値は受け取る)。
type SysInfo interface {
	Hostname() (string, error)
	HomeDir() (string, error)
	Pid() int
	IsTTY() bool
}

// IdGen は outside の口と同じ形 (ID は受け取る)。
type IdGen interface{ NewID() string }

// Good は 口から時刻を受ける (lint が通すべき形)。
func Good(c Clock) time.Time { return c.Now() }

// Label は 機体の値と ID を口から受ける (lint が通すべき形)。
func Label(s SysInfo, ids IdGen) (string, error) {
	host, err := s.Hostname()
	if err != nil {
		return "", err
	}
	home, err := s.HomeDir()
	if err != nil {
		return "", err
	}
	if s.IsTTY() && s.Pid() > 0 {
		return host + ":" + home, nil
	}
	return host + ":" + ids.NewID(), nil
}

// Last は 名前が rand の 自作の package を呼ぶ (標準の乱数ではないので lint が通すべき形)。
func Last(n int) int { return rand.Intn(n) }
