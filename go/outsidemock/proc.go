package outsidemock

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/aimglow/outside/go"
)

// Proc は mock の子 process。起こされた内容を記録し 決めた結果を返す。program を起こさない。
type Proc struct {
	recorder
	programs map[string]func(outside.Cmd) outside.Result
	mu       sync.Mutex
	paths    map[string]string
	cmds     []outside.Cmd
}

// Cmds は 起こされた Cmd を 起こされた順に返す (Stdin・Env・Dir まで test から読める)。
func (p *Proc) Cmds() []outside.Cmd {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]outside.Cmd, len(p.cmds))
	for i, c := range p.cmds {
		c.Args = append([]string(nil), c.Args...)
		c.Env = append([]string(nil), c.Env...)
		c.Stdin = append([]byte(nil), c.Stdin...)
		out[i] = c
	}
	return out
}

// NewProc は programs (名前 → 結果を決める関数) を宣言した Proc を返す。
// 関数が nil の名前は 空の結果 (exit 0) を返す。
func NewProc(programs map[string]func(outside.Cmd) outside.Result) *Proc {
	m := make(map[string]func(outside.Cmd) outside.Result, len(programs))
	for k, v := range programs {
		m[k] = v
	}
	return &Proc{programs: m}
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

// LookPath は WithPaths で渡した path を返す。NewProc で宣言していない名前は ErrUndeclaredProgram。
func (p *Proc) LookPath(name string) (string, error) {
	p.record("LookPath", name)
	if _, ok := p.programs[name]; !ok {
		return "", fmt.Errorf("%w: %s", outside.ErrUndeclaredProgram, name)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
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
func (p *Proc) Run(ctx context.Context, cmd outside.Cmd) (outside.Result, error) {
	p.record("Run", append([]string{cmd.Name}, cmd.Args...)...)
	p.mu.Lock()
	p.cmds = append(p.cmds, cmd)
	p.mu.Unlock()
	fn, ok := p.programs[cmd.Name]
	if !ok {
		return outside.Result{}, fmt.Errorf("%w: %s", outside.ErrUndeclaredProgram, cmd.Name)
	}
	if err := ctx.Err(); err != nil {
		return outside.Result{}, err
	}
	if fn == nil {
		return outside.Result{}, nil
	}
	return fn(cmd), nil
}
