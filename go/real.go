package outside

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aimglow/outside/go/internal/rule"
)

// この file が 本物の実装 (real)。外を直に呼ぶのは ここだけ。

type realClock struct{}

// NewClock は 実の時計を返す。
func NewClock() Clock { return realClock{} }

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

type realEnv struct{}

// NewEnv は process の環境変数を読む口を返す。
func NewEnv() Env { return realEnv{} }

func (realEnv) Lookup(name string) (string, bool) { return os.LookupEnv(name) }

type realFS struct{ root string }

// NewFS は root を根とする口を返す。root が dir として在らなければ 誤り。
func NewFS(root string) (FS, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("outside: FS の根が無い: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("outside: FS の根が dir ではない: %s", root)
	}
	return realFS{root: root}, nil
}

// open は path を確かめて 根を開く。symlink で根の外へ出る path は os.Root が止める。
func (f realFS) open(p string) (*os.Root, string, error) {
	c, err := rule.CleanRel(p)
	if err != nil {
		return nil, "", err
	}
	r, err := os.OpenRoot(f.root)
	if err != nil {
		return nil, "", err
	}
	return r, c, nil
}

func (f realFS) ReadFile(p string) ([]byte, error) {
	r, c, err := f.open(p)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return r.ReadFile(c)
}

func (f realFS) WriteFile(p string, data []byte) error {
	r, c, err := f.open(p)
	if err != nil {
		return err
	}
	defer r.Close()
	if dir := path.Dir(c); dir != "." {
		if err := r.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return r.WriteFile(c, data, 0o644)
}

func (f realFS) ReadDir(p string) ([]DirEntry, error) {
	r, c, err := f.open(p)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	entries, err := fs.ReadDir(r.FS(), c)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, DirEntry{Name: e.Name(), IsDir: e.IsDir()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f realFS) Stat(p string) (FileInfo, error) {
	r, c, err := f.open(p)
	if err != nil {
		return FileInfo{}, err
	}
	defer r.Close()
	info, err := r.Stat(c)
	if err != nil {
		return FileInfo{}, err
	}
	if info.IsDir() {
		return FileInfo{Name: path.Base(c), IsDir: true}, nil
	}
	return FileInfo{Name: path.Base(c), Size: info.Size()}, nil
}

func (f realFS) Remove(p string) error {
	r, c, err := f.open(p)
	if err != nil {
		return err
	}
	defer r.Close()
	if c == "." {
		return fmt.Errorf("outside: 根は消せない")
	}
	return r.Remove(c)
}

// gitLocalEnv は git の向き先を変える環境変数。宣言した木だけを見るため 子 process へ渡さない。
var gitLocalEnv = map[string]bool{
	"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true, "GIT_COMMON_DIR": true,
	"GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_NAMESPACE": true, "GIT_PREFIX": true,
}

type realGit struct {
	dir string
	env []string
}

// NewGit は dir の木の git に問う口を返す。extraEnv は git の子 process に足す環境変数 (NAME=値)。
func NewGit(dir string, extraEnv ...string) Git {
	return realGit{dir: dir, env: extraEnv}
}

// run は git を起こし 標準出力と exit の番号を返す。
func (g realGit) run(args ...string) (string, int, error) {
	cmd := exec.Command("git", append([]string{"-C", g.dir}, args...)...)
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); !gitLocalEnv[name] {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, g.env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() >= 0 {
		return stdout.String(), ee.ExitCode(), fmt.Errorf("outside: git %s: exit %d: %s",
			strings.Join(args, " "), ee.ExitCode(), strings.TrimSpace(stderr.String()))
	}
	if err != nil {
		return "", -1, fmt.Errorf("outside: git が起こせない: %w", err)
	}
	return stdout.String(), 0, nil
}

// splitZ は NUL 区切りの出力を 名前の順の一覧にする。
func splitZ(out string) []string {
	names := []string{}
	for _, n := range strings.Split(out, "\x00") {
		if n != "" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

func (g realGit) Head() (string, error) {
	out, _, err := g.run("rev-parse", "--verify", "HEAD")
	return strings.TrimSpace(out), err
}

func (g realGit) ListFiles() ([]string, error) {
	out, _, err := g.run("ls-files", "-z")
	if err != nil {
		return nil, err
	}
	return splitZ(out), nil
}

func (g realGit) MergeBase(a, b string) (string, error) {
	out, _, err := g.run("merge-base", a, b)
	return strings.TrimSpace(out), err
}

func (g realGit) DiffNames(from, to string) ([]string, error) {
	out, _, err := g.run("diff", "--name-only", "-z", from, to)
	if err != nil {
		return nil, err
	}
	return splitZ(out), nil
}

func (g realGit) IsAncestor(ancestor, descendant string) (bool, error) {
	_, code, err := g.run("merge-base", "--is-ancestor", ancestor, descendant)
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, err
	}
}

func (g realGit) Resolve(ref string) (string, error) {
	out, _, err := g.run("rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	return strings.TrimSpace(out), err
}

func (g realGit) ListAt(rev, dir string) ([]string, error) {
	c, err := rule.CleanRel(dir)
	if err != nil {
		return nil, err
	}
	spec := rev + ":" + c
	if c == "." {
		spec = rev + ":"
	}
	out, code, err := g.run("ls-tree", "-z", "--name-only", spec)
	if code == 128 {
		if _, e := g.Resolve(rev); e != nil {
			return nil, e
		}
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return splitZ(out), nil
}

func (g realGit) ReadAt(rev, p string) ([]byte, error) {
	c, err := rule.CleanRel(p)
	if err != nil {
		return nil, err
	}
	if _, err := g.Resolve(rev); err != nil {
		return nil, err
	}
	if _, code, _ := g.run("cat-file", "-e", rev+":"+c); code != 0 {
		return nil, fmt.Errorf("outside: %s:%s: %w", rev, c, fs.ErrNotExist)
	}
	out, _, err := g.run("cat-file", "blob", rev+":"+c)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

type realProc struct{ programs map[string]string }

// NewProc は programs (名前 → 実行 file の path) に在る program だけを起こす口を返す。
// path が空の名前は PATH から探す。
func NewProc(programs map[string]string) Proc {
	m := make(map[string]string, len(programs))
	for k, v := range programs {
		m[k] = v
	}
	return realProc{programs: m}
}

// Run は 子 process を起こす。子は 親の環境変数に cmd.Env を足した物を受ける。
func (p realProc) Run(ctx context.Context, c Cmd) (Result, error) {
	file, ok := p.programs[c.Name]
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrUndeclaredProgram, c.Name)
	}
	if file == "" {
		found, err := exec.LookPath(c.Name)
		if err != nil {
			return Result{}, fmt.Errorf("outside: program %s が見つからない: %w", c.Name, err)
		}
		file = found
	}
	cmd := exec.CommandContext(ctx, file, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(os.Environ(), c.Env...)
	if c.Stdin != nil {
		cmd.Stdin = bytes.NewReader(c.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() >= 0 && ctx.Err() == nil {
		res.ExitCode = ee.ExitCode()
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("outside: program %s: %w", c.Name, err)
	}
	return res, nil
}

type realWorkdir struct {
	root string
	made []string
}

// NewWorkdir は root を置き場とする口を返す。root が空か dir として在らなければ ErrNoWorkdirRoot。
// system の一時置き場へは 逃がさない。
func NewWorkdir(root string) (Workdir, error) {
	if root == "" {
		return nil, fmt.Errorf("%w: 置き場が宣言されていない", ErrNoWorkdirRoot)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNoWorkdirRoot, root)
	}
	return &realWorkdir{root: root}, nil
}

func (w *realWorkdir) New(name string) (string, error) {
	if err := rule.WorkdirName(name); err != nil {
		return "", err
	}
	p := filepath.Join(w.root, name)
	if err := os.Mkdir(p, 0o700); err != nil {
		return "", fmt.Errorf("outside: 作業場が作れない: %w", err)
	}
	w.made = append(w.made, p)
	return p, nil
}

func (w *realWorkdir) Path() string {
	if len(w.made) == 0 {
		return ""
	}
	return w.made[len(w.made)-1]
}

func (w *realWorkdir) FS(p string) (FS, error) {
	for _, m := range w.made {
		if m == p {
			return NewFS(p)
		}
	}
	return nil, fmt.Errorf("outside: この口が作っていない作業場: %s", p)
}

func (w *realWorkdir) Cleanup() error {
	var errs []error
	for _, p := range w.made {
		errs = append(errs, os.RemoveAll(p))
	}
	w.made = nil
	return errors.Join(errs...)
}

// RunTool は ツールの入口。declPath の宣言 (outside.toml) を読み 環境を確かめ、
// 合えば real の Set で fn を呼ぶ。合わなければ fn を呼ばず 足りない物を名乗る誤りを返す。
// 宣言の相対 path の基準は requires_git_worktree がtrue なら git の木の根、false なら 宣言の file の dir。
func RunTool(declPath string, fn func(Set) error) error {
	data, err := os.ReadFile(declPath)
	if err != nil {
		return fmt.Errorf("%w: 宣言が読めない: %v", ErrUndetermined, err)
	}
	decl, err := ParseDecl(data)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(filepath.Dir(declPath))
	if err != nil {
		return fmt.Errorf("%w: 宣言の置き場が決まらない: %v", ErrUndetermined, err)
	}
	if decl.RequiresGitWorktree {
		out, _, err := realGit{dir: root}.run("rev-parse", "--show-toplevel")
		if err != nil {
			return &MismatchError{Missing: []string{fmt.Sprintf("git の木 (%v)", err)}}
		}
		root = strings.TrimSpace(out)
	}
	fsys, err := NewFS(root)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUndetermined, err)
	}
	programs := make(map[string]string, len(decl.Programs))
	for name := range decl.Programs {
		programs[name] = ""
	}
	base := Base{
		Clock: NewClock(),
		Env:   NewEnv(),
		FS:    fsys,
		Git:   NewGit(root),
		Proc:  NewProc(programs),
		Workdir: func(r string) (Workdir, error) {
			if !filepath.IsAbs(r) {
				r = filepath.Join(root, r)
			}
			return NewWorkdir(r)
		},
		EnsureWorkdir: func(r string) (Workdir, error) {
			p := filepath.Join(root, r)
			if err := os.MkdirAll(p, 0o755); err != nil {
				return nil, fmt.Errorf("%w: %s を作れない: %v", ErrNoWorkdirRoot, p, err)
			}
			return NewWorkdir(p)
		},
	}
	return RunToolWith(decl, base, fn)
}
