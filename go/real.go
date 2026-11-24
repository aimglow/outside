package outside

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/aimglow/outside/go/internal/rule"
)

// この file が 本物の実装 (real)。外を直に呼ぶのは ここだけ。

type realClock struct{}

// NewClock は 実の時計を返す。
func NewClock() Clock { return realClock{} }

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) Sleep(d time.Duration)                  { time.Sleep(d) }
func (realClock) NewTimer(d time.Duration) Timer         { return realTimer{time.NewTimer(d)} }
func (realClock) NewTicker(d time.Duration) Ticker       { return realTicker{time.NewTicker(d)} }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

type realTimer struct{ t *time.Timer }

func (t realTimer) C() <-chan time.Time        { return t.t.C }
func (t realTimer) Stop() bool                 { return t.t.Stop() }
func (t realTimer) Reset(d time.Duration) bool { return t.t.Reset(d) }

type realTicker struct{ t *time.Ticker }

func (t realTicker) C() <-chan time.Time { return t.t.C }
func (t realTicker) Stop()               { t.t.Stop() }

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

func (f realFS) Open(p string) (io.ReadCloser, error) {
	r, c, err := f.open(p)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	// Root から開いた file は Root を閉じても読める (file は 自分の記述子を持つ)。
	file, err := r.Open(c)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if info.IsDir() {
		file.Close()
		return nil, &fs.PathError{Op: "open", Path: c, Err: errors.New("dir は読めない")}
	}
	return file, nil
}

func (f realFS) AppendFile(p string, data []byte) error {
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
	file, err := r.OpenFile(c, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (f realFS) MkdirAll(p string) error {
	r, c, err := f.open(p)
	if err != nil {
		return err
	}
	defer r.Close()
	return r.MkdirAll(c, 0o755)
}

func (f realFS) Lstat(p string) (FileInfo, error) {
	r, c, err := f.open(p)
	if err != nil {
		return FileInfo{}, err
	}
	defer r.Close()
	info, err := r.Lstat(c)
	if err != nil {
		return FileInfo{}, err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return FileInfo{Name: path.Base(c), IsSymlink: true}, nil
	case info.IsDir():
		return FileInfo{Name: path.Base(c), IsDir: true}, nil
	}
	return FileInfo{Name: path.Base(c), Size: info.Size()}, nil
}

func (f realFS) Rename(from, to string) error {
	r, cf, err := f.open(from)
	if err != nil {
		return err
	}
	defer r.Close()
	ct, err := rule.CleanRel(to)
	if err != nil {
		return err
	}
	if cf == "." || ct == "." {
		return renameErr(cf, ct, errors.New("根は移せない"))
	}
	src, err := r.Lstat(cf)
	if err != nil {
		return err
	}
	if cf == ct {
		return nil
	}
	if src.IsDir() && rule.Within(ct, cf) {
		return renameErr(cf, ct, errors.New("dir を 自分の下へは移せない"))
	}
	// 移し先の dir は 置き換えない (os.Rename と同じ。OS ごとの違いを出さない)。
	if dst, err := r.Lstat(ct); err == nil && dst.IsDir() {
		return renameErr(cf, ct, errors.New("移し先に dir が在る"))
	}
	return r.Rename(cf, ct)
}

// renameErr は Rename が 自分で止めた誤り (mock と同じ形)。
func renameErr(from, to string, err error) error {
	return &fs.PathError{Op: "rename", Path: from + " -> " + to, Err: err}
}

func (f realFS) RemoveAll(p string) error {
	r, c, err := f.open(p)
	if err != nil {
		return err
	}
	defer r.Close()
	if c == "." {
		return &fs.PathError{Op: "removeall", Path: c, Err: errors.New("根は消せない")}
	}
	return r.RemoveAll(c)
}

type realSysInfo struct{}

// NewSysInfo は 動いている機体と process の値を返す口を返す。
func NewSysInfo() SysInfo { return realSysInfo{} }

func (realSysInfo) Hostname() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("outside: host の名前が引けない: %w", err)
	}
	if name == "" {
		return "", fmt.Errorf("outside: host の名前が空")
	}
	return name, nil
}

func (realSysInfo) HomeDir() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("outside: home の dir が引けない: %w", err)
	}
	if dir == "" {
		return "", fmt.Errorf("outside: home の dir が空")
	}
	return dir, nil
}

func (realSysInfo) Pid() int { return os.Getpid() }

func (realSysInfo) IsTTY() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type realRandom struct{}

// NewRandom は crypto/rand の乱数を返す口を返す。
func NewRandom() Random { return realRandom{} }

// Read は p を全部埋める (crypto/rand.Read は 誤りを返さない決まり)。
func (realRandom) Read(p []byte) (int, error) { return rand.Read(p) }

type realIdGen struct{}

// NewIdGen は 呼ぶたびに 違う ID (UUID v4 の形、乱数は crypto/rand) を返す口を返す。
func NewIdGen() IdGen { return realIdGen{} }

func (realIdGen) NewID() string {
	var b [16]byte
	// crypto/rand.Read は 誤りを返さない (読めない時は process を止める) 決まり。
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40 // 版 4
	b[8] = b[8]&0x3f | 0x80 // variant 10
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// worktreeRoot は dir から上へ .git (dir でも file でも) を探し 最初に在った dir を返す。git の program は起こさない。
// ceiling が空でなければ その dir より上へは探さない (ceiling 自身は探す)。見つからなければ 誤り。
func worktreeRoot(dir, ceiling string) (string, error) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if ceiling != "" {
		if ceiling, err = filepath.Abs(ceiling); err != nil {
			return "", err
		}
	}
	for {
		_, err := os.Stat(filepath.Join(d, ".git"))
		if err == nil {
			return d, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%s の .git が確かめられない: %w", d, err)
		}
		parent := filepath.Dir(d)
		if d == ceiling || parent == d {
			return "", fmt.Errorf("%s から上に .git が無い", dir)
		}
		d = parent
	}
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

// LookPath は 宣言の path を返す。path が空の名前は PATH から探す。実行 file が無ければ exec.ErrNotFound の仲間の誤り。
func (p realProc) LookPath(name string) (string, error) {
	file, ok := p.programs[name]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUndeclaredProgram, name)
	}
	if file == "" {
		found, err := exec.LookPath(name)
		if err != nil {
			// exec.LookPath の誤りは errors.Is(err, exec.ErrNotFound) で判る。
			return "", fmt.Errorf("outside: program %s が見つからない: %w", name, err)
		}
		return found, nil
	}
	// 宣言の path は 実行できる file かを確かめる (path を持つ名前は exec.LookPath が PATH を探さず その file を見る)。
	if _, err := exec.LookPath(file); err != nil {
		return "", fmt.Errorf("outside: program %s (%s) が見つからない: %w: %v", name, file, exec.ErrNotFound, err)
	}
	return file, nil
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
// 宣言の相対 path の基準は requires_git_worktree が true なら git の木の根 (宣言の dir から上へ .git を探す。git の program は起こさない)、
// false なら 宣言の file の dir。
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
	var worktree func() (string, error)
	if decl.RequiresGitWorktree {
		top, err := worktreeRoot(root, "")
		if err != nil {
			return &MismatchError{Missing: []string{fmt.Sprintf("git の木 (%v)", err)}}
		}
		root = top
		worktree = func() (string, error) { return top, nil }
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
		Clock:    NewClock(),
		Env:      NewEnv(),
		FS:       fsys,
		Proc:     NewProc(programs),
		SysInfo:  NewSysInfo(),
		IdGen:    NewIdGen(),
		Random:   NewRandom(),
		Worktree: worktree,
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
