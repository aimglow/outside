package outsidemock

import (
	"fmt"
	"path"
	"sync"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/internal/rule"
)

// Workdir は memory の上の作業場。disk に何も作らない。
type Workdir struct {
	recorder
	mu   sync.Mutex
	root string
	made []string
}

// NewWorkdir は root を置き場とする Workdir を返す。
// root が空か exists が偽 (置き場が在らない) なら ErrNoWorkdirRoot。
func NewWorkdir(root string, exists bool) (*Workdir, error) {
	if root == "" {
		return nil, fmt.Errorf("%w: 置き場が宣言されていない", outside.ErrNoWorkdirRoot)
	}
	if !exists {
		return nil, fmt.Errorf("%w: %s", outside.ErrNoWorkdirRoot, root)
	}
	return &Workdir{root: root}, nil
}

func (w *Workdir) New(name string) (string, error) {
	w.record("New", name)
	if err := rule.WorkdirName(name); err != nil {
		return "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	p := path.Join(w.root, name)
	for _, m := range w.made {
		if m == p {
			return "", fmt.Errorf("outsidemock: 作業場が既に在る: %s", p)
		}
	}
	w.made = append(w.made, p)
	return p, nil
}

func (w *Workdir) Path() string {
	w.record("Path")
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.made) == 0 {
		return ""
	}
	return w.made[len(w.made)-1]
}

func (w *Workdir) Cleanup() error {
	w.record("Cleanup")
	w.mu.Lock()
	defer w.mu.Unlock()
	w.made = nil
	return nil
}

// Live は 今 在る作業場の path を 作った順に返す。
func (w *Workdir) Live() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.made...)
}
