package outsidemock

import "fmt"

// Pair は 2 つの commit の組 (問いの引数の順)。
type Pair struct{ A, B string }

// GitValues は 偽の Git が返す値。test が値として宣言する。
type GitValues struct {
	Head       string
	Files      []string
	MergeBase  map[Pair]string
	DiffNames  map[Pair][]string
	IsAncestor map[Pair]bool
}

// Git は 渡した値を返すmock。repo を作らない。宣言に無い問いは 誤り。
type Git struct {
	recorder
	v GitValues
}

// NewGit は v を返す Git を作る。
func NewGit(v GitValues) *Git { return &Git{v: v} }

func (g *Git) Head() (string, error) {
	g.record("Head")
	if g.v.Head == "" {
		return "", fmt.Errorf("outsidemock: head が宣言されていない")
	}
	return g.v.Head, nil
}

func (g *Git) ListFiles() ([]string, error) {
	g.record("ListFiles")
	return append([]string{}, g.v.Files...), nil
}

func (g *Git) MergeBase(a, b string) (string, error) {
	g.record("MergeBase", a, b)
	sha, ok := g.v.MergeBase[Pair{a, b}]
	if !ok {
		return "", fmt.Errorf("outsidemock: merge-base %s %s が宣言されていない", a, b)
	}
	return sha, nil
}

func (g *Git) DiffNames(from, to string) ([]string, error) {
	g.record("DiffNames", from, to)
	names, ok := g.v.DiffNames[Pair{from, to}]
	if !ok {
		return nil, fmt.Errorf("outsidemock: 差分 %s %s が宣言されていない", from, to)
	}
	return append([]string{}, names...), nil
}

func (g *Git) IsAncestor(ancestor, descendant string) (bool, error) {
	g.record("IsAncestor", ancestor, descendant)
	is, ok := g.v.IsAncestor[Pair{ancestor, descendant}]
	if !ok {
		return false, fmt.Errorf("outsidemock: 祖先の問い %s %s が宣言されていない", ancestor, descendant)
	}
	return is, nil
}
