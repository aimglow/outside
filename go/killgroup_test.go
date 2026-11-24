package outside_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// TC-OUT-PROC-KILLGROUP (mock): ctx を受ける関数を宣言でき ctx が 関数の前か後に終わっていれば Killed = true と ctx の誤り。
// Cmds() に KillGroup・KillGrace が残る。実の process・実時間を使わない。(real は killgroup_unix_test.go)
func TestProc_TC_OUT_PROC_KILLGROUP_Mock(t *testing.T) {
	waits := func(ctx context.Context, c outside.Cmd) (outside.Result, error) {
		<-ctx.Done()
		return outside.Result{Stdout: []byte("partial")}, nil
	}
	quick := func(_ context.Context, c outside.Cmd) (outside.Result, error) {
		return outside.Result{ExitCode: 2, Stdout: []byte(c.Args[0])}, nil
	}
	p := outsidemock.NewProc(map[string]func(outside.Cmd) outside.Result{"old": outsidemock.Fixed(outside.Result{ExitCode: 1})}).
		WithFunc("svc", waits).
		WithFunc("quick", quick).
		WithPaths(map[string]string{"svc": "/opt/bin/svc"})
	grace := 300 * time.Millisecond

	// 関数の中で ctx が終わる (関数の後の確かめ)。
	ctx, cancel := context.WithCancel(context.Background())
	go func() { cancel() }()
	res, err := p.Run(ctx, outside.Cmd{Name: "svc", Args: []string{"serve"}, KillGroup: true, KillGrace: grace})
	if !res.Killed || !errors.Is(err, context.Canceled) || string(res.Stdout) != "partial" {
		t.Errorf("関数の中で切った Run = %+v, %v", res, err)
	}
	// 関数の前に ctx が終わっている。
	res, err = p.Run(ctx, outside.Cmd{Name: "quick", Args: []string{"x"}})
	if !res.Killed || !errors.Is(err, context.Canceled) {
		t.Errorf("終わった ctx の Run = %+v, %v", res, err)
	}
	// 自分で終える関数は Killed = false。NewProc の関数も そのまま動く。
	live := context.Background()
	if res, err := p.Run(live, outside.Cmd{Name: "quick", Args: []string{"y"}, KillGroup: true}); err != nil || res.Killed || res.ExitCode != 2 || string(res.Stdout) != "y" {
		t.Errorf("自分で終える Run = %+v, %v", res, err)
	}
	if res, err := p.Run(live, outside.Cmd{Name: "old"}); err != nil || res.Killed || res.ExitCode != 1 {
		t.Errorf("NewProc の関数の Run = %+v, %v", res, err)
	}
	if got, err := p.LookPath("svc"); err != nil || got != "/opt/bin/svc" {
		t.Errorf("WithFunc で宣言した名前の LookPath = %q, %v", got, err)
	}

	cmds := p.Cmds()
	if len(cmds) != 4 {
		t.Fatalf("記録の数 = %d", len(cmds))
	}
	if !cmds[0].KillGroup || cmds[0].KillGrace != grace || cmds[1].KillGroup || !cmds[2].KillGroup || cmds[2].KillGrace != 0 {
		t.Errorf("記録の KillGroup・KillGrace = %+v", cmds)
	}
	want := []outsidemock.Call{
		{Method: "Run", Args: []string{"svc", "serve"}},
		{Method: "Run", Args: []string{"quick", "x"}},
		{Method: "Run", Args: []string{"quick", "y"}},
		{Method: "Run", Args: []string{"old"}},
		{Method: "LookPath", Args: []string{"svc"}},
	}
	if got := p.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("記録 = %+v", got)
	}
}
