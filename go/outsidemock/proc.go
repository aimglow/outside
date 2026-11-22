package outsidemock

import (
	"context"
	"fmt"

	"github.com/aimglow/outside/go"
)

// Proc は 偽の子 process。起こされた内容を記録し 決めた結果を返す。program を起こさない。
type Proc struct {
	recorder
	programs map[string]func(outside.Cmd) outside.Result
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

// Fixed は 引数に依らず r を返す関数を作る。
func Fixed(r outside.Result) func(outside.Cmd) outside.Result {
	return func(outside.Cmd) outside.Result { return r }
}

// Run は 記録して 決めた結果を返す。記録の Args は 名前に続けて 引数。
func (p *Proc) Run(ctx context.Context, cmd outside.Cmd) (outside.Result, error) {
	p.record("Run", append([]string{cmd.Name}, cmd.Args...)...)
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
