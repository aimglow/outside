package outsidemock

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/aimglow/outside/go"
)

// Func は ctx を受けて 結果を決める関数 (WithFunc で宣言する)。実の process・実時間を使わない。
type Func func(ctx context.Context, c outside.Cmd) (outside.Result, error)

// Proc は mock の子 process。起こされた内容を記録し 決めた結果を返す。program を起こさない。
type Proc struct {
	recorder
	mu       sync.Mutex
	programs map[string]Func
	paths    map[string]string
	cmds     []outside.Cmd
}

// Cmds は 起こされた Cmd を 起こされた順に返す (Stdin・Env・Unset・Dir・KillGroup・KillGrace まで test から読める)。
func (p *Proc) Cmds() []outside.Cmd {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]outside.Cmd, len(p.cmds))
	for i, c := range p.cmds {
		c.Args = append([]string(nil), c.Args...)
		c.Env = append([]string(nil), c.Env...)
		c.Unset = append([]string(nil), c.Unset...)
		c.Stdin = append([]byte(nil), c.Stdin...)
		out[i] = c
	}
	return out
}

// NewProc は programs (名前 → 結果を決める関数) を宣言した Proc を返す。
// 関数が nil の名前は 空の結果 (exit 0) を返す。
func NewProc(programs map[string]func(outside.Cmd) outside.Result) *Proc {
	m := make(map[string]Func, len(programs))
	for k, v := range programs {
		if v == nil {
			m[k] = nil
			continue
		}
		fn := v
		m[k] = func(_ context.Context, c outside.Cmd) (outside.Result, error) { return fn(c), nil }
	}
	return &Proc{programs: m}
}

// WithFunc は name を ctx を受ける関数 fn で宣言し p を返す (同じ名前の前の宣言は 置き換わる)。
// fn が nil なら 空の結果 (exit 0) を返す。fn の前か後に ctx が終わっていれば Run は Killed = true と ctx の誤りを返す
// (fn は ctx.Done() を待って 止められる子を表せる)。
func (p *Proc) WithFunc(name string, fn Func) *Proc {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.programs[name] = fn
	return p
}

// WithPaths は LookPath が返す path (名前 → 実行 file の path) を渡し p を返す。前に渡した表は 置き換わる。
// 表に無い名前の LookPath は 見つからない誤り (errors.Is(err, exec.ErrNotFound))。
func (p *Proc) WithPaths(paths map[string]string) *Proc {
	m := make(map[string]string, len(paths))
	for k, v := range paths {
		m[k] = v
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paths = m
	return p
}

// LookPath は WithPaths で渡した path を返す。NewProc・WithFunc で宣言していない名前は ErrUndeclaredProgram。
func (p *Proc) LookPath(name string) (string, error) {
	p.record("LookPath", name)
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.programs[name]; !ok {
		return "", fmt.Errorf("%w: %s", outside.ErrUndeclaredProgram, name)
	}
	if file, ok := p.paths[name]; ok && file != "" {
		return file, nil
	}
	return "", fmt.Errorf("outsidemock: program %s の path が渡されていない: %w", name, exec.ErrNotFound)
}

// Fixed は 引数に依らず r を返す関数を作る。
func Fixed(r outside.Result) func(outside.Cmd) outside.Result {
	return func(outside.Cmd) outside.Result { return r }
}

// Run は 記録して 決めた結果を返す。記録の Args は 名前に続けて 引数。
// ctx が 関数の前か後に終わっていれば Result.Killed = true と ctx の誤りを返す。
func (p *Proc) Run(ctx context.Context, cmd outside.Cmd) (outside.Result, error) {
	p.record("Run", append([]string{cmd.Name}, cmd.Args...)...)
	p.mu.Lock()
	p.cmds = append(p.cmds, cmd)
	fn, ok := p.programs[cmd.Name]
	p.mu.Unlock()
	if !ok {
		return outside.Result{}, fmt.Errorf("%w: %s", outside.ErrUndeclaredProgram, cmd.Name)
	}
	if cmd.StdoutLimit < 0 {
		return outside.Result{}, fmt.Errorf("outsidemock: StdoutLimit が負 (%d)", cmd.StdoutLimit)
	}
	if err := ctx.Err(); err != nil {
		return outside.Result{Killed: true}, err
	}
	if fn == nil {
		return outside.Result{}, nil
	}
	res, err := fn(ctx, cmd)
	res = limitStdout(res, cmd.StdoutLimit)
	if ctxErr := ctx.Err(); ctxErr != nil {
		res.Killed = true
		return res, ctxErr
	}
	return res, err
}

// limitStdout は real と同じく Stdout を 先頭の limit bytes までにし 切った事を StdoutTruncated で名乗る (0 は 上限なし)。
func limitStdout(res outside.Result, limit int) outside.Result {
	if limit > 0 && len(res.Stdout) > limit {
		res.Stdout = append([]byte(nil), res.Stdout[:limit]...)
		res.StdoutTruncated = true
	}
	return res
}
