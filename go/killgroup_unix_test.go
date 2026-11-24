//go:build unix

package outside_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/aimglow/outside/go"
)

// TC-OUT-PROC-KILLGROUP (real): helper (この test の実行 file) が 孫を起こして pid を書き 待つ。
// ctx を切ると 孫まで止まり Result.Killed = true。

func init() {
	helperExtra["spawn"] = helperSpawn
	helperExtra["sleep"] = helperSleep
}

// helperSpawn は args = [pid の file, term|ignore]。孫 (sleep 役) を起こし 孫の支度が済んだら
// 「自分の pid 孫の pid」を file に書き 孫を待つ。孫は 標準出力を受け継ぐ (pipe を握ったまま残る孫を表す)。
func helperSpawn(args []string) {
	pidFile, mode := args[0], args[1]
	if mode == "ignore" {
		signal.Ignore(syscall.SIGTERM)
	}
	self, err := os.Executable()
	if err != nil {
		os.Exit(70)
	}
	ready := pidFile + ".grandchild"
	gc := exec.Command(self, "outside-helper", "sleep", ready, mode)
	gc.Stdout, gc.Stderr = os.Stdout, os.Stderr
	if err := gc.Start(); err != nil {
		os.Exit(71)
	}
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	writeAtomic(pidFile, fmt.Sprintf("%d %d", os.Getpid(), gc.Process.Pid))
	_ = gc.Wait()
}

// helperSleep は args = [支度の file, term|ignore]。SIGTERM の扱いを決めてから 支度の file を書き 眠り続ける。
func helperSleep(args []string) {
	if args[1] == "ignore" {
		signal.Ignore(syscall.SIGTERM)
	}
	writeAtomic(args[0], "ready")
	for {
		time.Sleep(time.Hour)
	}
}

// writeAtomic は 一時の名前に書いてから付け替える (読み手が 書きかけを読まない)。
func writeAtomic(p, s string) {
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(s), 0o644); err != nil {
		os.Exit(72)
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Exit(73)
	}
}

// alive は pid に signal 0 が届くか (止まっていれば ESRCH)。
func alive(pid int) bool { return !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }

// waitGone は pid が止まるのを 上限まで待つ (孫は 親が消えた後 init が回収する)。
func waitGone(pid int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for alive(pid) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
	return true
}

type runOut struct {
	res outside.Result
	err error
}

// startSpawn は helper の spawn 役を KillGroup で起こし 子と孫の pid が書かれるまで待つ。
// 終わりに group と孫へ SIGKILL を送り 孫を残さない。
func startSpawn(t *testing.T, ctx context.Context, mode string, grace time.Duration) (child, grandchild int, done <-chan runOut) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(testDir(t), "pids")
	p := outside.NewProc(map[string]string{"helper": self})
	ch := make(chan runOut, 1)
	go func() {
		res, err := p.Run(ctx, outside.Cmd{
			Name: "helper", Args: []string{"outside-helper", "spawn", pidFile, mode},
			KillGroup: true, KillGrace: grace,
		})
		ch <- runOut{res, err}
	}()
	deadline := time.Now().Add(30 * time.Second)
	for {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			if _, err := fmt.Sscanf(string(data), "%d %d", &child, &grandchild); err != nil {
				t.Fatalf("pid の file = %q: %v", data, err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper が pid を書かない")
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-child, syscall.SIGKILL)
		_ = syscall.Kill(grandchild, syscall.SIGKILL)
	})
	return child, grandchild, ch
}

func TestProc_TC_OUT_PROC_KILLGROUP_Real(t *testing.T) {
	t.Run("SIGTERM で孫まで止まる", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		child, grandchild, done := startSpawn(t, ctx, "term", 0)
		if !alive(grandchild) {
			t.Fatal("切る前に 孫が居ない")
		}
		cancel()
		out := <-done
		if !out.res.Killed || !errors.Is(out.err, context.Canceled) {
			t.Errorf("Run = %+v, %v (want Killed と context.Canceled)", out.res, out.err)
		}
		for _, pid := range []int{child, grandchild} {
			if !waitGone(pid, 5*time.Second) {
				t.Errorf("pid %d が 止まっていない", pid)
			}
		}
	})
	t.Run("SIGTERM を無視する子も KillGrace の後に止まる", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		grace := 300 * time.Millisecond
		child, grandchild, done := startSpawn(t, ctx, "ignore", grace)
		began := time.Now()
		cancel()
		out := <-done
		took := time.Since(began)
		if !out.res.Killed || !errors.Is(out.err, context.Canceled) {
			t.Errorf("Run = %+v, %v (want Killed と context.Canceled)", out.res, out.err)
		}
		// 既定の 5s ではなく 渡した猶予で止まる。猶予より前には SIGKILL しない。
		if took < grace || took > 4*time.Second {
			t.Errorf("止まるまで %v (want 猶予 %v 以上 4s 以下)", took, grace)
		}
		for _, pid := range []int{child, grandchild} {
			if !waitGone(pid, 5*time.Second) {
				t.Errorf("pid %d が 止まっていない", pid)
			}
		}
	})
	t.Run("短い期限で切る", func(t *testing.T) {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ready := filepath.Join(testDir(t), "ready")
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		p := outside.NewProc(map[string]string{"helper": self})
		res, err := p.Run(ctx, outside.Cmd{
			Name: "helper", Args: []string{"outside-helper", "sleep", ready, "term"},
			KillGroup: true, KillGrace: time.Second,
		})
		if !res.Killed || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Run = %+v, %v (want Killed と context.DeadlineExceeded)", res, err)
		}
	})
	t.Run("KillGroup = false でも ctx の終わりで止めた時は Killed", func(t *testing.T) {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ready := filepath.Join(testDir(t), "ready")
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		p := outside.NewProc(map[string]string{"helper": self})
		res, err := p.Run(ctx, outside.Cmd{Name: "helper", Args: []string{"outside-helper", "sleep", ready, "term"}})
		if !res.Killed || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Run = %+v, %v (want Killed と context.DeadlineExceeded)", res, err)
		}
		// 起こす前に ctx が終わっていても Killed。
		res, err = p.Run(ctx, outside.Cmd{Name: "helper", Args: []string{"outside-helper", "echo", "x"}, KillGroup: true})
		if !res.Killed || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("終わった ctx の Run = %+v, %v", res, err)
		}
	})
	t.Run("自分で終えた子は Killed = false", func(t *testing.T) {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		p := outside.NewProc(map[string]string{"helper": self})
		ctx := context.Background()
		res, err := p.Run(ctx, outside.Cmd{Name: "helper", Args: []string{"outside-helper", "echo", "hi"}, KillGroup: true})
		if err != nil || res.Killed || string(res.Stdout) != "hi" {
			t.Errorf("Run(echo) = %+v, %v", res, err)
		}
		res, err = p.Run(ctx, outside.Cmd{Name: "helper", Args: []string{"outside-helper", "exit", "3"}, KillGroup: true})
		if err != nil || res.Killed || res.ExitCode != 3 {
			t.Errorf("Run(exit 3) = %+v, %v", res, err)
		}
	})
}
