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
	"strings"
	"sync"
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

// realFS は root を根とする FS。filePerm・dirPerm は この FS が新しく作る file と dir の権限
// (umask に依らず この値になるよう 作った物にだけ Chmod を当てる。既に在る物の権限は変えない)。
type realFS struct {
	root     string
	filePerm fs.FileMode
	dirPerm  fs.FileMode
}

// NewFS は root を根とする口を返す。作る file は 0644・dir は 0755。root が dir として在らなければ 誤り。
func NewFS(root string) (FS, error) {
	return newRealFS(root, 0o644, 0o755)
}

// NewPrivateFS は root を根とする 秘密を置く口を返す。作る file は 0600・dir は 0700 (同じ機体の他の user に読ませない)。
// root が dir として在らなければ 誤り。root 自身の権限は変えない。
func NewPrivateFS(root string) (FS, error) {
	return newRealFS(root, 0o600, 0o700)
}

// NewReadOnlyFS は root を根とする 読み取り専用の file を置く口を返す。作る file は 0444・dir は 0755
// (完了の印など 一度書いたら上書きさせない物)。その file への 2 度目の書き込みは fs.ErrPermission。root 自身の権限は変えない。
func NewReadOnlyFS(root string) (FS, error) {
	return newRealFS(root, 0o444, 0o755)
}

func newRealFS(root string, filePerm, dirPerm fs.FileMode) (FS, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("outside: FS の根が無い: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("outside: FS の根が dir ではない: %s", root)
	}
	return realFS{root: root, filePerm: filePerm, dirPerm: dirPerm}, nil
}

// mkdirAll は 整えた c と その親の dir を 無ければ作る。新しく作った dir にだけ dirPerm を当てる。
// 途中の名前が dir 以外 (file・dir を指さない symlink) なら 誤り。symlink は 根の中で辿る (os.Root が止める)。
func (f realFS) mkdirAll(r *os.Root, c string) error {
	if c == "." {
		return nil
	}
	parts := strings.Split(c, "/")
	for i := range parts {
		d := strings.Join(parts[:i+1], "/")
		err := r.Mkdir(d, f.dirPerm)
		if err == nil {
			if err := r.Chmod(d, f.dirPerm); err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, serr := r.Stat(d)
		if serr != nil {
			return serr
		}
		if !info.IsDir() {
			return &fs.PathError{Op: "mkdir", Path: d, Err: errors.New("dir ではない物が在る")}
		}
	}
	return nil
}

// openForWrite は c を書くために開く。無ければ filePerm で作り 権限を当てる (新しく作った時だけ)。
// flag は O_WRONLY に足す物 (O_TRUNC か O_APPEND)。c が 先の無い symlink なら 先を新しく作る。
func (f realFS) openForWrite(r *os.Root, c string, flag int) (*os.File, error) {
	file, err := r.OpenFile(c, os.O_WRONLY|os.O_CREATE|os.O_EXCL|flag, f.filePerm)
	if err == nil {
		return f.fixNew(file)
	}
	if !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	// 何かが在る (file・dir・symlink)。在る file は 権限を変えずに開く。
	file, err = r.OpenFile(c, os.O_WRONLY|flag, 0)
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		return file, err
	}
	// 先の無い symlink: 辿った先を作る (先は 新しい file)。
	file, err = r.OpenFile(c, os.O_WRONLY|os.O_CREATE|flag, f.filePerm)
	if err != nil {
		return nil, err
	}
	return f.fixNew(file)
}

// fixNew は 新しく作った file に filePerm を当てる (umask で削られた bit を戻し 決めた権限にする)。
func (f realFS) fixNew(file *os.File) (*os.File, error) {
	if err := file.Chmod(f.filePerm); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
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
	if err := f.mkdirAll(r, path.Dir(c)); err != nil {
		return err
	}
	file, err := f.openForWrite(r, c, os.O_TRUNC)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
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
		return FileInfo{Name: path.Base(c), IsDir: true, Mode: info.Mode().Perm()}, nil
	}
	return FileInfo{Name: path.Base(c), Size: info.Size(), Mode: info.Mode().Perm()}, nil
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
	if err := f.mkdirAll(r, path.Dir(c)); err != nil {
		return err
	}
	file, err := f.openForWrite(r, c, os.O_APPEND)
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
	return f.mkdirAll(r, c)
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
		// symlink の権限の bit は OS ごとに違い 使われない ∴ 0。
		return FileInfo{Name: path.Base(c), IsSymlink: true}, nil
	case info.IsDir():
		return FileInfo{Name: path.Base(c), IsDir: true, Mode: info.Mode().Perm()}, nil
	}
	return FileInfo{Name: path.Base(c), Size: info.Size(), Mode: info.Mode().Perm()}, nil
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

// Symlink は 同じ dir に 一時の名前で symlink を作り Rename で link へ付け替える (途中で link が無い時を作らない)。
// 付け替えに失敗したら 一時の物を消す。target は 文字列のまま渡す (os.Root.Symlink は target を確かめない)。
func (f realFS) Symlink(target, link string) error {
	r, c, err := f.open(link)
	if err != nil {
		return err
	}
	defer r.Close()
	if c == "." {
		return symlinkErr(c, errors.New("根は link にできない"))
	}
	if target == "" {
		return symlinkErr(c, errors.New("target が空"))
	}
	if info, err := r.Lstat(c); err == nil {
		if info.Mode()&fs.ModeSymlink == 0 {
			if info.IsDir() {
				return symlinkErr(c, errors.New("link に dir が在る"))
			}
			return symlinkErr(c, errors.New("link に file が在る"))
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	tmp := path.Join(path.Dir(c), "."+path.Base(c)+".outside-link-"+hex.EncodeToString(b[:]))
	if err := r.Symlink(target, tmp); err != nil {
		return err
	}
	if err := r.Rename(tmp, c); err != nil {
		_ = r.Remove(tmp)
		return err
	}
	return nil
}

// symlinkErr は Symlink が 自分で止めた誤り (mock と同じ形)。
func symlinkErr(link string, err error) error {
	return &fs.PathError{Op: "symlink", Path: link, Err: err}
}

func (f realFS) Readlink(p string) (string, error) {
	r, c, err := f.open(p)
	if err != nil {
		return "", err
	}
	defer r.Close()
	return r.Readlink(c)
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

// nonEmpty は 引けた値が空でなければ返す。誤りか空なら what を名乗る誤り。
func nonEmpty(what string, v string, err error) (string, error) {
	if err != nil {
		return "", fmt.Errorf("outside: %s が引けない: %w", what, err)
	}
	if v == "" {
		return "", fmt.Errorf("outside: %s が空", what)
	}
	return v, nil
}

func (realSysInfo) ConfigDir() (string, error) {
	v, err := os.UserConfigDir()
	return nonEmpty("user の設定の置き場", v, err)
}

func (realSysInfo) CacheDir() (string, error) {
	v, err := os.UserCacheDir()
	return nonEmpty("user の cache の置き場", v, err)
}

func (realSysInfo) Getwd() (string, error) {
	v, err := os.Getwd()
	return nonEmpty("作業 dir", v, err)
}

func (realSysInfo) Pid() int { return os.Getpid() }

func (realSysInfo) Ppid() int { return os.Getppid() }

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
// ctx の終わりで止めた時は Result.Killed = true で 誤りは ctx の誤りを包む。
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
	cmd.Env = append(withoutNames(os.Environ(), c.Unset), c.Env...)
	if c.Stdin != nil {
		cmd.Stdin = bytes.NewReader(c.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	// stop は ctx の終わりで 止めに入ったかを持つ (exec の Cancel は 別の goroutine から呼ばれる)。
	stop := &procStop{}
	var group *killGroup
	if c.KillGroup {
		grace := c.KillGrace
		if grace <= 0 {
			grace = defaultKillGrace
		}
		g, err := newKillGroup(cmd, grace, stop)
		if err != nil {
			return Result{}, fmt.Errorf("outside: program %s: %w", c.Name, err)
		}
		group = g
	} else {
		cmd.Cancel = func() error {
			err := cmd.Process.Kill()
			if err == nil {
				stop.mark()
			}
			return err
		}
	}
	err := cmd.Run()
	if group != nil {
		group.finish()
	}
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	// 起こす前に ctx が終わっていた時 (cmd.Process が無い) も ctx の終わりで止めた事にする。
	if ctxErr := ctx.Err(); ctxErr != nil && (stop.marked() || cmd.Process == nil) {
		res.Killed = true
		if err != nil && errors.Is(err, ctxErr) {
			return res, fmt.Errorf("outside: program %s: %w", c.Name, err)
		}
		return res, fmt.Errorf("outside: program %s を ctx の終わりで止めた: %w (%v)", c.Name, ctxErr, err)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() >= 0 {
		res.ExitCode = ee.ExitCode()
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("outside: program %s: %w", c.Name, err)
	}
	return res, nil
}

// procStop は ctx の終わりで 止めに入った (signal を送った) かを持つ。
type procStop struct {
	mu   sync.Mutex
	done bool
	at   time.Time
}

func (s *procStop) mark() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.done {
		s.done, s.at = true, time.Now()
	}
}

func (s *procStop) marked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done
}

// since は 止めに入った時刻を返す。入っていなければ false。
func (s *procStop) since() (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.at, s.done
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

// withoutNames は NAME=値 の並びから names の名の物を外す。
func withoutNames(env, names []string) []string {
	if len(names) == 0 {
		return env
	}
	drop := make(map[string]bool, len(names))
	for _, n := range names {
		drop[n] = true
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if name, _, _ := strings.Cut(kv, "="); !drop[name] {
			out = append(out, kv)
		}
	}
	return out
}
