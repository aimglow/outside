package outsidemock

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/aimglow/outside/go/internal/rule"
)

// Pair は 2 つの commit の組 (問いの引数の順)。
type Pair struct{ A, B string }

// GitValues は mock の Git が返す値。test が値として宣言する。
type GitValues struct {
	Head       string
	Files      []string
	MergeBase  map[Pair]string
	DiffNames  map[Pair][]string
	IsAncestor map[Pair]bool
	// Refs は ref → sha。Resolve が引く。
	Refs map[string]string
	// Trees は rev → その木の file (相対 path → 中身)。ListAt と ReadAt が引く。
	Trees map[string]map[string]string
}

// Git は 渡した値を返す mock。repo を作らない。宣言に無い問いは 誤り。
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

func (g *Git) Resolve(ref string) (string, error) {
	g.record("Resolve", ref)
	sha, ok := g.v.Refs[ref]
	if !ok {
		return "", fmt.Errorf("outsidemock: ref %s が宣言されていない", ref)
	}
	return sha, nil
}

func (g *Git) tree(rev string) (map[string]string, error) {
	t, ok := g.v.Trees[rev]
	if !ok {
		return nil, fmt.Errorf("outsidemock: rev %s の木が宣言されていない", rev)
	}
	return t, nil
}

func (g *Git) ListAt(rev, dir string) ([]string, error) {
	g.record("ListAt", rev, dir)
	c, err := rule.CleanRel(dir)
	if err != nil {
		return nil, err
	}
	t, err := g.tree(rev)
	if err != nil {
		return nil, err
	}
	prefix := c + "/"
	if c == "." {
		prefix = ""
	}
	seen := map[string]bool{}
	names := []string{}
	for p := range t {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok || rest == "" {
			continue
		}
		name, _, _ := strings.Cut(rest, "/")
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (g *Git) ReadAt(rev, p string) ([]byte, error) {
	g.record("ReadAt", rev, p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return nil, err
	}
	t, err := g.tree(rev)
	if err != nil {
		return nil, err
	}
	body, ok := t[c]
	if !ok {
		return nil, fmt.Errorf("outsidemock: %s:%s: %w", rev, c, fs.ErrNotExist)
	}
	return []byte(body), nil
}
