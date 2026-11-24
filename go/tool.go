package outside

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/aimglow/outside/go/internal/rule"
)

// exit の番号。判定不能は ツールの合否と分ける。
const (
	ExitOK           = 0
	ExitFail         = 1
	ExitUndetermined = 3
)

// ExitCode は RunTool の戻りを exit の番号へ直す。
func ExitCode(err error) int {
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, ErrUndetermined):
		return ExitUndetermined
	default:
		return ExitFail
	}
}

// Decl は tool-environment の宣言 (outside.toml)。
type Decl struct {
	// RequiresGitWorktree は git の木の中で動く事を求める。
	RequiresGitWorktree bool `toml:"requires_git_worktree"`
	// Programs は 起こしてよい program の 名前 → 版。版が空なら 起こせる事だけを確かめる。
	Programs map[string]string `toml:"programs"`
	// WorkdirRoot は 作業場の置き場。根からの相対 path か $NAME (環境変数の値) か $NAME/<相対 path>。空なら 作業場を使わない。
	WorkdirRoot string `toml:"workdir_root"`
	// Env は 読んでよい環境変数の名前。
	Env []string `toml:"env"`
	// FSRoots は 読み書きしてよい根 (根からの相対 path)。
	FSRoots []string `toml:"fs_roots"`
}

// ParseDecl は TOML の宣言を読む。知らない項目は 誤り。
func ParseDecl(data []byte) (Decl, error) {
	var d Decl
	md, err := toml.Decode(string(data), &d)
	if err != nil {
		return Decl{}, fmt.Errorf("%w: 宣言が読めない: %v", ErrUndetermined, err)
	}
	if u := md.Undecoded(); len(u) > 0 {
		return Decl{}, fmt.Errorf("%w: 宣言に知らない項目: %v", ErrUndetermined, u)
	}
	return d, nil
}

// MismatchError は 環境が宣言に合わない事を表す。Missing が 足りない物の一覧。
type MismatchError struct {
	Missing []string
}

func (e *MismatchError) Error() string {
	return "outside: 環境が tool-environment の宣言に合わない: " + strings.Join(e.Missing, "; ")
}

// Is は 判定不能の仲間として扱わせる。
func (e *MismatchError) Is(target error) bool { return target == ErrUndetermined }

// Base は 宣言で絞る前の外。RunToolWith が 宣言を確かめ Set を作る材料にする。
type Base struct {
	Clock Clock
	Env   Env
	// FS は 宣言の相対 path の基準 (repo の根) を根とする。
	FS   FS
	Proc Proc
	// SysInfo と IdGen は そのまま Set へ渡る (宣言で絞らない)。
	SysInfo SysInfo
	IdGen   IdGen
	// Worktree は requires_git_worktree の時に git の木の根を返す。nil か 誤りなら git の木が足りないと名乗る。
	Worktree func() (root string, err error)
	// Workdir は 置き場を受けて 作業場の口を返す。置き場が無ければ 誤り。
	Workdir func(root string) (Workdir, error)
	// EnsureWorkdir は repo の中の相対 path の置き場を 無ければ作って 作業場の口を返す。
	// 宣言が repo の中の相対 path の時だけ使う ($NAME の置き場は 機体が用意する物ゆえ Workdir で 無ければ誤り)。
	// nil なら Workdir と同じ。
	EnsureWorkdir func(root string) (Workdir, error)
}

var versionRe = regexp.MustCompile(`\d+(\.\d+)*`)

// versionOK は program の名乗った版が 宣言の版と同じか その下位の版かを返す。
func versionOK(want, got string) bool {
	return got == want || strings.HasPrefix(got, want+".")
}

// RunToolWith は base を宣言と突き合わせ、合えば 宣言の範囲に絞った Set で fn を呼ぶ。
// 合わなければ fn を呼ばず MismatchError を返す。終わりに 作業場を消す。
func RunToolWith(decl Decl, base Base, fn func(Set) error) (err error) {
	var missing []string

	if decl.RequiresGitWorktree {
		if base.Worktree == nil {
			missing = append(missing, "git の木 (確かめる口が無い)")
		} else if _, e := base.Worktree(); e != nil {
			missing = append(missing, fmt.Sprintf("git の木 (%v)", e))
		}
	}

	names := make([]string, 0, len(decl.Programs))
	for name := range decl.Programs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		want := decl.Programs[name]
		res, e := base.Proc.Run(context.Background(), Cmd{Name: name, Args: []string{"--version"}})
		if e != nil {
			missing = append(missing, fmt.Sprintf("program %s (起こせない: %v)", name, e))
			continue
		}
		if want == "" {
			continue
		}
		got := versionRe.FindString(string(res.Stdout) + string(res.Stderr))
		if !versionOK(want, got) {
			missing = append(missing, fmt.Sprintf("program %s の版 %s (在る版: %q)", name, want, got))
		}
	}

	var wd Workdir = noWorkdir{}
	if decl.WorkdirRoot != "" {
		if w, e := resolveWorkdir(decl.WorkdirRoot, base); e != nil {
			missing = append(missing, e.Error())
		} else {
			wd = w
		}
	}

	roots := make([]string, 0, len(decl.FSRoots))
	for _, r := range decl.FSRoots {
		c, e := rule.CleanRel(r)
		if e != nil {
			missing = append(missing, fmt.Sprintf("fs_roots %s (根の外)", r))
			continue
		}
		if info, e := base.FS.Stat(c); e != nil || !info.IsDir {
			missing = append(missing, fmt.Sprintf("fs_roots %s (dir が無い)", r))
			continue
		}
		roots = append(roots, c)
	}

	if len(missing) > 0 {
		return &MismatchError{Missing: missing}
	}

	set := Set{
		Clock:   base.Clock,
		Env:     limitEnv{inner: base.Env, names: toSet(decl.Env)},
		FS:      limitFS{inner: base.FS, roots: roots},
		Proc:    limitProc{inner: base.Proc, names: toSet(names)},
		Workdir: wd,
		SysInfo: base.SysInfo,
		IdGen:   base.IdGen,
	}
	defer func() {
		if e := wd.Cleanup(); e != nil && err == nil {
			err = fmt.Errorf("outside: 作業場が消せない: %w", e)
		}
	}()
	return fn(set)
}

// resolveWorkdir は workdir_root を置き場へ直し 作業場の口を作る。誤りの文は 足りない物を名乗る。
func resolveWorkdir(root string, base Base) (Workdir, error) {
	open := base.Workdir
	if ref, ok := strings.CutPrefix(root, "$"); ok {
		name, sub, _ := strings.Cut(ref, "/")
		v, found := base.Env.Lookup(name)
		if !found || v == "" {
			return nil, fmt.Errorf("環境変数 %s (workdir_root)", name)
		}
		root = v
		if sub != "" {
			c, err := rule.CleanRel(sub)
			if err != nil {
				return nil, fmt.Errorf("workdir_root %s (環境変数の下の外)", ref)
			}
			root = path.Join(v, c)
		}
	} else {
		c, err := rule.CleanRel(root)
		if err != nil {
			return nil, fmt.Errorf("workdir_root %s (根の外)", root)
		}
		root = c
		if base.EnsureWorkdir != nil {
			open = base.EnsureWorkdir
		}
	}
	w, err := open(root)
	if err != nil {
		return nil, fmt.Errorf("workdir_root %s (置き場が無い: %v)", root, err)
	}
	return w, nil
}

func toSet(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// limitEnv は 宣言した名前だけを読ませる。宣言に無い名前は 無い と返す。
type limitEnv struct {
	inner Env
	names map[string]bool
}

func (e limitEnv) Lookup(name string) (string, bool) {
	if !e.names[name] {
		return "", false
	}
	return e.inner.Lookup(name)
}

// limitFS は 宣言した根の下だけを触らせる。
type limitFS struct {
	inner FS
	roots []string
}

func (f limitFS) check(p string) (string, error) {
	c, err := rule.CleanRel(p)
	if err != nil {
		return "", err
	}
	for _, r := range f.roots {
		if r == "." || c == r || strings.HasPrefix(c, r+"/") {
			return c, nil
		}
	}
	return "", fmt.Errorf("%w: %s (fs_roots に無い)", ErrOutsideRoot, p)
}

func (f limitFS) ReadFile(p string) ([]byte, error) {
	c, err := f.check(p)
	if err != nil {
		return nil, err
	}
	return f.inner.ReadFile(c)
}

func (f limitFS) WriteFile(p string, data []byte) error {
	c, err := f.check(p)
	if err != nil {
		return err
	}
	return f.inner.WriteFile(c, data)
}

func (f limitFS) ReadDir(p string) ([]DirEntry, error) {
	c, err := f.check(p)
	if err != nil {
		return nil, err
	}
	return f.inner.ReadDir(c)
}

func (f limitFS) Stat(p string) (FileInfo, error) {
	c, err := f.check(p)
	if err != nil {
		return FileInfo{}, err
	}
	return f.inner.Stat(c)
}

func (f limitFS) Remove(p string) error {
	c, err := f.check(p)
	if err != nil {
		return err
	}
	return f.inner.Remove(c)
}

// limitProc は 宣言した program だけを起こさせる。
type limitProc struct {
	inner Proc
	names map[string]bool
}

func (p limitProc) Run(ctx context.Context, cmd Cmd) (Result, error) {
	if !p.names[cmd.Name] {
		return Result{}, fmt.Errorf("%w: %s", ErrUndeclaredProgram, cmd.Name)
	}
	return p.inner.Run(ctx, cmd)
}

// noWorkdir は 置き場を宣言していない時の口。作業場を作らせない。
type noWorkdir struct{}

func (noWorkdir) New(string) (string, error) {
	return "", fmt.Errorf("%w: workdir_root が宣言されていない", ErrNoWorkdirRoot)
}
func (noWorkdir) Path() string   { return "" }
func (noWorkdir) Cleanup() error { return nil }
func (noWorkdir) FS(string) (FS, error) {
	return nil, fmt.Errorf("%w: workdir_root が宣言されていない", ErrNoWorkdirRoot)
}
