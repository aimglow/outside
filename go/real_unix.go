//go:build unix

package outside

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// この file も 本物の実装 (real)。process group を持つ OS (unix) で Cmd.KillGroup を扱う。

// killGroup は 新しい process group で起こした子と その孫を ctx の終わりで止める。
type killGroup struct {
	cmd   *exec.Cmd
	grace time.Duration
	stop  *procStop
}

// groupGoneWait は SIGKILL の後に group が消えるのを待つ上限。
const groupGoneWait = time.Second

// newKillGroup は cmd を 新しい process group で起こす形にする。
// ctx が終わると Cancel が group 全体へ SIGTERM を送り、WaitDelay (= grace) が 子と pipe を待つ上限になる。
// grace を過ぎて group に まだ居る物は finish が SIGKILL で止める。
func newKillGroup(cmd *exec.Cmd, grace time.Duration, stop *procStop) (*killGroup, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		if err == nil {
			stop.mark()
		}
		return err
	}
	cmd.WaitDelay = grace
	return &killGroup{cmd: cmd, grace: grace, stop: stop}, nil
}

// finish は Run が子を待ち終えた後に呼ぶ。止めに入っていれば SIGTERM から grace の間 group が消えるのを待ち、
// 残っていれば group へ SIGKILL を送り 消えるのを待つ (上限 groupGoneWait)。
func (g *killGroup) finish() {
	at, ok := g.stop.since()
	if !ok || g.cmd.Process == nil {
		return
	}
	pgid := g.cmd.Process.Pid
	if waitGroupGone(pgid, time.Until(at.Add(g.grace))) {
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	waitGroupGone(pgid, groupGoneWait)
}

// waitGroupGone は d の間 group pgid が消えるのを待つ。消えたら true。
func waitGroupGone(pgid int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}
