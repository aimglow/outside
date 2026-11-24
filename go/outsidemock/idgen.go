package outsidemock

import (
	"fmt"
	"sync"
)

// IdGen は id-0001・id-0002 … と 呼んだ順に決まった ID を返す mock。mock ごとに 1 から始まる。並行に呼んでも安全。
type IdGen struct {
	recorder
	mu sync.Mutex
	n  int
}

// NewIdGen は id-0001 から返す IdGen を作る。
func NewIdGen() *IdGen { return &IdGen{} }

func (g *IdGen) NewID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	id := fmt.Sprintf("id-%04d", g.n)
	g.record("NewID", id)
	return id
}
