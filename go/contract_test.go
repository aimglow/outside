package outside_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// TC-OUT-CONTRACT: 口ごとの契約の test を real と mock の両方に掛ける。
// real に掛ける時は test が自分で作った物 (testDir が repo の .work/ の下に作った dir・自分で起こした子 process) だけを触る。

// TestMain は 引数が outside-helper の時だけ 子 process 役として振る舞う (Proc の契約の相手)。
func TestMain(m *testing.M) {
	if len(os.Args) >= 3 && os.Args[1] == "outside-helper" {
		helperMain(os.Args[2:])
	}
	os.Exit(m.Run())
}

// helperExtra は OS に依る helper の役 (名前 → 振る舞い)。build tag の付いた test の file が init で足す。
var helperExtra = map[string]func(args []string){}

func helperMain(args []string) {
	if fn, ok := helperExtra[args[0]]; ok {
		fn(args[1:])
		os.Exit(0)
	}
	switch args[0] {
	case "echo":
		fmt.Print(strings.Join(args[1:], " "))
		os.Exit(0)
	case "exit":
		n, _ := strconv.Atoi(args[1])
		os.Exit(n)
	case "cat":
		b, _ := io.ReadAll(os.Stdin)
		os.Stdout.Write(b)
		os.Exit(0)
	case "getenv":
		if v, ok := os.LookupEnv(args[1]); ok {
			fmt.Print("set:" + v)
		} else {
			fmt.Print("unset")
		}
		os.Exit(0)
	}
	os.Exit(64)
}

// 契約: Now は戻らない。0 以下の Sleep は 待たずに戻る。
func clockContract(t *testing.T, c outside.Clock) {
	t.Helper()
	before := c.Now()
	c.Sleep(0)
	c.Sleep(-time.Hour)
	after := c.Now()
	if after.Before(before) {
		t.Errorf("Now が戻った: %v → %v", before, after)
	}
}

func TestContract_TC_OUT_CONTRACT_Clock(t *testing.T) {
	t.Run("real", func(t *testing.T) { clockContract(t, outside.NewClock()) })
	t.Run("mock", func(t *testing.T) {
		clockContract(t, outsidemock.NewClock(time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)))
	})
}

const (
	envSet    = "OUTSIDE_CONTRACT_SET"
	envEmpty  = "OUTSIDE_CONTRACT_EMPTY"
	envAbsent = "OUTSIDE_CONTRACT_ABSENT"
)

// 契約: 在る名前は値、空の値も 在る、無い名前は 無い。
func envContract(t *testing.T, e outside.Env) {
	t.Helper()
	if v, ok := e.Lookup(envSet); !ok || v != "value" {
		t.Errorf("Lookup(%s) = %q, %v", envSet, v, ok)
	}
	if v, ok := e.Lookup(envEmpty); !ok || v != "" {
		t.Errorf("Lookup(%s) = %q, %v", envEmpty, v, ok)
	}
	if v, ok := e.Lookup(envAbsent); ok || v != "" {
		t.Errorf("Lookup(%s) = %q, %v", envAbsent, v, ok)
	}
}

func TestContract_TC_OUT_CONTRACT_Env(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		// test が自分で置いた変数だけを読む。無い名前は 自分で消して 無い事を保証する。
		t.Setenv(envSet, "value")
		t.Setenv(envEmpty, "")
		t.Setenv(envAbsent, "x")
		if err := os.Unsetenv(envAbsent); err != nil {
			t.Fatal(err)
		}
		envContract(t, outside.NewEnv())
	})
	t.Run("mock", func(t *testing.T) {
		envContract(t, outsidemock.NewEnv(map[string]string{envSet: "value", envEmpty: ""}))
	})
}

// 契約: 書いた中身が読める・一覧は名前の順・無い path は ErrNotExist・根の外は ErrOutsideRoot。
// f は 空の根から始まる。abs は 根の外の絶対 path。
func fsContract(t *testing.T, f outside.FS, abs string) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.WriteFile("a/b.txt", []byte("old")))
	must(f.WriteFile("a/b.txt", []byte("hello")))
	must(f.WriteFile("a/c/d.txt", nil))
	if data, err := f.ReadFile("a/b.txt"); err != nil || string(data) != "hello" {
		t.Errorf("ReadFile = %q, %v", data, err)
	}
	if data, err := f.ReadFile("./a/../a/b.txt"); err != nil || string(data) != "hello" {
		t.Errorf("根の中に収まる path の ReadFile = %q, %v", data, err)
	}
	if info, err := f.Stat("a/b.txt"); err != nil || info != (outside.FileInfo{Name: "b.txt", Size: 5, Mode: 0o644}) {
		t.Errorf("Stat(file) = %+v, %v", info, err)
	}
	if info, err := f.Stat("a"); err != nil || info != (outside.FileInfo{Name: "a", IsDir: true, Mode: 0o755}) {
		t.Errorf("Stat(dir) = %+v, %v", info, err)
	}
	if got, err := f.ReadDir("a"); err != nil ||
		!reflect.DeepEqual(got, []outside.DirEntry{{Name: "b.txt"}, {Name: "c", IsDir: true}}) {
		t.Errorf("ReadDir(a) = %+v, %v", got, err)
	}
	if got, err := f.ReadDir("."); err != nil || !reflect.DeepEqual(got, []outside.DirEntry{{Name: "a", IsDir: true}}) {
		t.Errorf("ReadDir(.) = %+v, %v", got, err)
	}

	// 無い path。
	if _, err := f.ReadFile("nope"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile(無い) = %v", err)
	}
	if _, err := f.Stat("nope"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(無い) = %v", err)
	}
	if _, err := f.ReadDir("nope"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadDir(無い) = %v", err)
	}
	if err := f.Remove("nope"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Remove(無い) = %v", err)
	}

	// 種類が合わない操作。
	if _, err := f.ReadFile("a"); err == nil {
		t.Error("dir の ReadFile が通った")
	}
	if _, err := f.ReadDir("a/b.txt"); err == nil {
		t.Error("file の ReadDir が通った")
	}
	if err := f.WriteFile("a", []byte("x")); err == nil {
		t.Error("dir への WriteFile が通った")
	}
	if err := f.WriteFile("a/b.txt/x", []byte("x")); err == nil {
		t.Error("file の下への WriteFile が通った")
	}
	if err := f.Remove("a"); err == nil {
		t.Error("空でない dir の Remove が通った")
	}
	if err := f.Remove("."); err == nil {
		t.Error("根の Remove が通った")
	}

	// 消す。
	must(f.Remove("a/b.txt"))
	if _, err := f.ReadFile("a/b.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("消した file の ReadFile = %v", err)
	}
	must(f.Remove("a/c/d.txt"))
	must(f.Remove("a/c"))
	if got, err := f.ReadDir("a"); err != nil || len(got) != 0 {
		t.Errorf("空にした dir の ReadDir = %+v, %v", got, err)
	}

	// 根の外。
	for _, p := range []string{"..", "../x", "a/../../x", abs, ""} {
		if _, err := f.ReadFile(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("ReadFile(%q) = %v", p, err)
		}
		if err := f.WriteFile(p, []byte("x")); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("WriteFile(%q) = %v", p, err)
		}
		if _, err := f.ReadDir(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("ReadDir(%q) = %v", p, err)
		}
		if _, err := f.Stat(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("Stat(%q) = %v", p, err)
		}
		if err := f.Remove(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("Remove(%q) = %v", p, err)
		}
	}
}

func TestContract_TC_OUT_CONTRACT_FS(t *testing.T) {
	// abs は test が自分で作った 根の外の file。読まれず 書き換わらない事も見る。
	newAbs := func(t *testing.T) string {
		abs := filepath.Join(testDir(t), "outside.txt")
		if err := os.WriteFile(abs, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		return abs
	}
	checkAbs := func(t *testing.T, abs string) {
		if data, err := os.ReadFile(abs); err != nil || string(data) != "keep" {
			t.Errorf("根の外の file が変わった: %q, %v", data, err)
		}
	}
	t.Run("real", func(t *testing.T) {
		f, err := outside.NewFS(testDir(t))
		if err != nil {
			t.Fatal(err)
		}
		abs := newAbs(t)
		fsContract(t, f, abs)
		checkAbs(t, abs)
	})
	t.Run("mock", func(t *testing.T) {
		f, err := outsidemock.NewFS(nil)
		if err != nil {
			t.Fatal(err)
		}
		abs := newAbs(t)
		fsContract(t, f, abs)
		checkAbs(t, abs)
	})
}

// real の FS は symlink で根の外へ出る path も止める (mock に symlink は無い)。
func TestRealFS_SymlinkEscape(t *testing.T) {
	root := testDir(t)
	secret := filepath.Join(testDir(t), "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	f, err := outside.NewFS(root)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := f.ReadFile("link"); err == nil {
		t.Errorf("symlink 越しに 根の外が読めた: %q", data)
	}
	if err := f.WriteFile("link", []byte("x")); err == nil {
		t.Error("symlink 越しに 根の外へ書けた")
	}
	if data, _ := os.ReadFile(secret); string(data) != "secret" {
		t.Errorf("根の外の file が変わった: %q", data)
	}
}

// real の FS は 根が無ければ 作れない。
func TestRealFS_MissingRoot(t *testing.T) {
	if _, err := outside.NewFS(filepath.Join(testDir(t), "nope")); err == nil {
		t.Error("無い根で FS が作れた")
	}
}

// 契約: Hostname と HomeDir は 空でない値か 誤り。Pid は 正の数。IsTTY は 呼べる。
func sysInfoContract(t *testing.T, s outside.SysInfo) {
	t.Helper()
	if v, err := s.Hostname(); err == nil && v == "" {
		t.Error("Hostname が 誤り無しで 空を返した")
	}
	if v, err := s.HomeDir(); err == nil && v == "" {
		t.Error("HomeDir が 誤り無しで 空を返した")
	}
	if pid := s.Pid(); pid <= 0 {
		t.Errorf("Pid = %d (want 正の数)", pid)
	}
	_ = s.IsTTY()
}

func TestContract_TC_OUT_CONTRACT_SysInfo(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		// 機体の値は 中身を比べない (機体ごとに違う)。契約の形だけを見る。
		sysInfoContract(t, outside.NewSysInfo())
	})
	t.Run("mock", func(t *testing.T) {
		s := outsidemock.NewSysInfo(outsidemock.SysInfoValues{Hostname: "host-a", HomeDir: "/home/a", Pid: 4242})
		sysInfoContract(t, s)
		if pid := s.Pid(); pid != 4242 {
			t.Errorf("mock の Pid = %d (want 渡した 4242)", pid)
		}
		// 渡していない値は 誤り (空で黙らない) も 契約の内。
		sysInfoContract(t, outsidemock.NewSysInfo(outsidemock.SysInfoValues{Pid: 1}))
	})
}

// uuidV4 は UUID v4 の文字列の形。
var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// 契約: 続けて呼んだ ID は 互いに違い 空でない。
func idGenContract(t *testing.T, g outside.IdGen) []string {
	t.Helper()
	seen := map[string]bool{}
	var ids []string
	for range 100 {
		id := g.NewID()
		if id == "" {
			t.Fatal("NewID が 空を返した")
		}
		if seen[id] {
			t.Fatalf("NewID が 同じ ID を返した: %q", id)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func TestContract_TC_OUT_CONTRACT_IdGen(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		for _, id := range idGenContract(t, outside.NewIdGen()) {
			if !uuidV4.MatchString(id) {
				t.Errorf("UUID v4 の形でない: %q", id)
			}
		}
	})
	t.Run("mock", func(t *testing.T) {
		idGenContract(t, outsidemock.NewIdGen())
	})
}

// 契約: Read は p を全部埋め n=len(p)・誤り無し。続けて読んだ 32 byte は 互いに違う。
func randomContract(t *testing.T, r outside.Random) {
	t.Helper()
	seen := map[string]bool{}
	for range 50 {
		p := make([]byte, 32)
		n, err := r.Read(p)
		if err != nil || n != len(p) {
			t.Fatalf("Read = %d, %v (want %d, nil)", n, err, len(p))
		}
		if seen[string(p)] {
			t.Fatalf("続けて読んだ 32 byte が 同じ: %x", p)
		}
		seen[string(p)] = true
	}
	if n, err := r.Read(nil); n != 0 || err != nil {
		t.Errorf("Read(nil) = %d, %v (want 0, nil)", n, err)
	}
}

func TestContract_TC_OUT_CONTRACT_Random(t *testing.T) {
	t.Run("real", func(t *testing.T) { randomContract(t, outside.NewRandom()) })
	t.Run("mock", func(t *testing.T) { randomContract(t, outsidemock.NewRandom([]byte("seed"))) })
}

// 契約: 宣言した helper は 起きて結果が返る。0 以外の exit は Result で返る。宣言に無い名前は 誤り。
func procContract(t *testing.T, p outside.Proc, prefix []string) {
	t.Helper()
	ctx := context.Background()
	args := func(a ...string) []string { return append(append([]string{}, prefix...), a...) }
	res, err := p.Run(ctx, outside.Cmd{Name: "helper", Args: args("echo", "hi", "there")})
	if err != nil || res.ExitCode != 0 || string(res.Stdout) != "hi there" {
		t.Errorf("echo = %+v, %v", res, err)
	}
	res, err = p.Run(ctx, outside.Cmd{Name: "helper", Args: args("cat"), Stdin: []byte("secret\nline2")})
	if err != nil || res.ExitCode != 0 || string(res.Stdout) != "secret\nline2" {
		t.Errorf("stdin = %+v, %v", res, err)
	}
	res, err = p.Run(ctx, outside.Cmd{Name: "helper", Args: args("cat")})
	if err != nil || res.ExitCode != 0 || len(res.Stdout) != 0 {
		t.Errorf("stdin 無し = %+v, %v", res, err)
	}
	res, err = p.Run(ctx, outside.Cmd{Name: "helper", Args: args("exit", "3")})
	if err != nil || res.ExitCode != 3 {
		t.Errorf("exit 3 = %+v, %v", res, err)
	}
	if _, err := p.Run(ctx, outside.Cmd{Name: "undeclared", Args: args("echo", "x")}); !errors.Is(err, outside.ErrUndeclaredProgram) {
		t.Errorf("宣言に無い program = %v", err)
	}
}

func TestContract_TC_OUT_CONTRACT_Proc(t *testing.T) {
	prefix := []string{"outside-helper"}
	t.Run("real", func(t *testing.T) {
		// 子 process は この test の実行 file 自身 (TestMain の helper 役)。
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		procContract(t, outside.NewProc(map[string]string{"helper": self}), prefix)
	})
	t.Run("mock", func(t *testing.T) {
		helper := func(c outside.Cmd) outside.Result {
			switch c.Args[1] {
			case "echo":
				return outside.Result{Stdout: []byte(strings.Join(c.Args[2:], " "))}
			case "exit":
				n, _ := strconv.Atoi(c.Args[2])
				return outside.Result{ExitCode: n}
			case "cat":
				return outside.Result{Stdout: append([]byte(nil), c.Stdin...)}
			}
			return outside.Result{ExitCode: 64}
		}
		p := outsidemock.NewProc(map[string]func(outside.Cmd) outside.Result{"helper": helper})
		procContract(t, p, prefix)
		var stdins []string
		for _, c := range p.Cmds() {
			stdins = append(stdins, string(c.Stdin))
		}
		if !reflect.DeepEqual(stdins, []string{"", "secret\nline2", "", "", ""}) {
			t.Errorf("記録の Stdin = %q", stdins)
		}
	})
}

// 契約: 作業場は 置き場の直下に出来る・同じ名前と使えない名前は 誤り・Cleanup で全て消える。
// exists は その path の作業場が 今 在るかを返す。
func workdirContract(t *testing.T, w outside.Workdir, root string, exists func(string) bool) {
	t.Helper()
	if got := w.Path(); got != "" {
		t.Errorf("作る前の Path = %q", got)
	}
	p1, err := w.New("job1")
	if err != nil {
		t.Fatal(err)
	}
	if rel, err := filepath.Rel(root, p1); err != nil || rel != "job1" {
		t.Errorf("作業場 %q が 置き場 %q の直下でない", p1, root)
	}
	if !exists(p1) || w.Path() != p1 {
		t.Errorf("作った作業場が無い: exists=%v Path=%q", exists(p1), w.Path())
	}
	if _, err := w.New("job1"); err == nil {
		t.Error("同じ名前の New が通った")
	}
	for _, name := range []string{"", ".", "..", "a/b", "../x"} {
		if _, err := w.New(name); err == nil {
			t.Errorf("New(%q) が通った", name)
		}
	}
	p2, err := w.New("job2")
	if err != nil {
		t.Fatal(err)
	}
	if w.Path() != p2 {
		t.Errorf("Path = %q (want %q)", w.Path(), p2)
	}
	f, err := w.FS(p2)
	if err != nil {
		t.Fatalf("作った作業場の FS = %v", err)
	}
	if err := f.WriteFile("m/001.sql", []byte("x")); err != nil {
		t.Errorf("作業場の FS に書けない: %v", err)
	}
	if got, err := f.ReadFile("m/001.sql"); err != nil || string(got) != "x" {
		t.Errorf("作業場の FS から読めない: %q, %v", got, err)
	}
	if _, err := f.ReadFile("../job1/x"); err == nil {
		t.Error("作業場の FS で 根の外が読めた")
	}
	if _, err := w.FS(filepath.Join(root, "not-made")); err == nil {
		t.Error("作っていない作業場の FS が通った")
	}
	if err := w.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if w.Path() != "" || exists(p1) || exists(p2) {
		t.Errorf("Cleanup の後に残った: Path=%q %v %v", w.Path(), exists(p1), exists(p2))
	}
}

func TestContract_TC_OUT_CONTRACT_Workdir(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		root := testDir(t)
		w, err := outside.NewWorkdir(root)
		if err != nil {
			t.Fatal(err)
		}
		workdirContract(t, w, root, func(p string) bool {
			info, err := os.Stat(p)
			return err == nil && info.IsDir()
		})
	})
	t.Run("mock", func(t *testing.T) {
		w, err := outsidemock.NewWorkdir("work", true)
		if err != nil {
			t.Fatal(err)
		}
		workdirContract(t, w, "work", func(p string) bool {
			for _, live := range w.Live() {
				if live == p {
					return true
				}
			}
			return false
		})
	})
}

// TC-OUT-WORKDIR-DECLARED: Workdir は 宣言した置き場の下にだけ作る。置き場が無ければ 誤り。
// (置き場の下に出来る事は TestContract_TC_OUT_CONTRACT_Workdir が real と mock の両方で見る。)
func TestWorkdir_TC_OUT_WORKDIR_DECLARED(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		parent := testDir(t)
		for _, root := range []string{"", filepath.Join(parent, "nope")} {
			if w, err := outside.NewWorkdir(root); !errors.Is(err, outside.ErrNoWorkdirRoot) {
				t.Errorf("NewWorkdir(%q) = %v, %v", root, w, err)
			}
		}
		// 置き場の代わりの場所が 作られていない。
		if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
			t.Errorf("置き場が無いのに 何か作られた: %v, %v", entries, err)
		}
		// file は 置き場にならない。
		file := filepath.Join(parent, "file")
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := outside.NewWorkdir(file); !errors.Is(err, outside.ErrNoWorkdirRoot) {
			t.Errorf("file を置き場にした NewWorkdir = %v", err)
		}
	})
	t.Run("mock", func(t *testing.T) {
		if _, err := outsidemock.NewWorkdir("", true); !errors.Is(err, outside.ErrNoWorkdirRoot) {
			t.Errorf("置き場の宣言が無い NewWorkdir = %v", err)
		}
		if _, err := outsidemock.NewWorkdir("work", false); !errors.Is(err, outside.ErrNoWorkdirRoot) {
			t.Errorf("置き場が在らない NewWorkdir = %v", err)
		}
	})
}

// testDir は この repo の .work/ の下に この test の dir を作り 終わりに消す (system の一時置き場を使わない)。
func testDir(t *testing.T) string {
	t.Helper()
	base, err := filepath.Abs(filepath.Join("..", ".work"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "go-outside-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// TC-OUT-CLOCK-TIMER (real と mock に共通の契約): 期限を過ぎた物だけが送る・Stop した物は送らない・Reset は掛け直す。
// advance は 時刻を進める (real = 実の Sleep / mock = Advance)。wait は 届くはずの値を待つ上限 (mock は 0 = 待たない)。
// unit は 期限の単位 (real は 短い実時間)。
func timerContract(t *testing.T, c outside.Clock, advance func(time.Duration), unit, wait time.Duration) {
	t.Helper()
	recv := func(ch <-chan time.Time, what string) time.Time {
		t.Helper()
		if wait == 0 {
			select {
			case v := <-ch:
				return v
			default:
				t.Fatalf("%s が届かない", what)
			}
		}
		select {
		case v := <-ch:
			return v
		case <-time.After(wait):
			t.Fatalf("%s が %v 待っても届かない", what, wait)
		}
		return time.Time{}
	}
	none := func(ch <-chan time.Time, what string) {
		t.Helper()
		select {
		case v := <-ch:
			t.Errorf("%s が届いた: %v", what, v)
		default:
		}
	}
	long := 1000 * unit
	start := c.Now()
	t1 := c.NewTimer(unit)
	stopped := c.NewTimer(long)
	pending := c.NewTimer(long)
	after := c.After(unit)
	advance(3 * unit)
	if v := recv(t1.C(), "期限を過ぎた Timer"); v.Before(start.Add(unit)) {
		t.Errorf("Timer の時刻 %v が 期限 %v より前", v, start.Add(unit))
	}
	if v := recv(after, "期限を過ぎた After"); v.Before(start.Add(unit)) {
		t.Errorf("After の時刻 %v が 期限 %v より前", v, start.Add(unit))
	}
	none(pending.C(), "期限前の Timer")
	if t1.Stop() {
		t.Error("発火済みの Timer の Stop が true")
	}
	if !stopped.Stop() {
		t.Error("動いている Timer の Stop が false")
	}
	if stopped.Stop() {
		t.Error("止めた Timer の 2 度目の Stop が true")
	}
	// Reset: 済んだ Timer は false、動いている Timer は true。どちらも 新しい期限で発火する。
	if t1.Reset(unit) {
		t.Error("発火済みの Timer の Reset が true")
	}
	if !pending.Reset(unit) {
		t.Error("動いている Timer の Reset が false")
	}
	before := c.Now()
	advance(3 * unit)
	if v := recv(t1.C(), "Reset した 済みの Timer"); v.Before(before.Add(unit)) {
		t.Errorf("Reset した Timer の時刻 %v が 新しい期限より前", v)
	}
	recv(pending.C(), "Reset した 動いている Timer")
	none(stopped.C(), "Stop した Timer")
	// 発火して 受けていない Timer を Stop すると 値は捨てられる (Stop の後に 前の期限の値は届かない)。
	unread := c.NewTimer(unit)
	advance(3 * unit)
	unread.Stop()
	none(unread.C(), "Stop の後の 前の期限の値")

	// Ticker: 間隔ごとに発火し Stop の後は 送らない。
	tk := c.NewTicker(unit)
	advance(3 * unit)
	first := recv(tk.C(), "Ticker の 1 回目")
	advance(3 * unit)
	if second := recv(tk.C(), "Ticker の 2 回目"); !second.After(first) {
		t.Errorf("Ticker の 2 回目 %v が 1 回目 %v の後でない", second, first)
	}
	tk.Stop()
	advance(3 * unit)
	none(tk.C(), "Stop した Ticker")
	stopped.Stop()
	pending.Stop()
	t1.Stop()
}

func TestContract_TC_OUT_CLOCK_TIMER(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		// 実の時計は 短い期限で 自分で掛けた物だけを見る。
		c := outside.NewClock()
		timerContract(t, c, c.Sleep, 20*time.Millisecond, 5*time.Second)
	})
	t.Run("mock", func(t *testing.T) {
		c := outsidemock.NewClock(time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC))
		began := time.Now()
		// mock は 1 時間の単位でも 実時間を待たない。
		timerContract(t, c, c.Advance, time.Hour, 0)
		if took := time.Since(began); took > 10*time.Second {
			t.Errorf("mock の契約に 実時間 %v かかった", took)
		}
	})
	t.Run("NewTicker の 0 以下は panic", func(t *testing.T) {
		for name, c := range map[string]outside.Clock{
			"real": outside.NewClock(),
			"mock": outsidemock.NewClock(time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)),
		} {
			func() {
				defer func() {
					if recover() == nil {
						t.Errorf("%s: NewTicker(0) が panic しない", name)
					}
				}()
				c.NewTicker(0)
			}()
		}
	})
}

// TC-OUT-FS-MORE: Open・AppendFile・MkdirAll・Lstat・Rename・RemoveAll の契約。f は 空の根から始まる。abs は 根の外の絶対 path。
func fsMoreContract(t *testing.T, f outside.FS, abs string) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	readAll := func(p string) (string, error) {
		rc, err := f.Open(p)
		if err != nil {
			return "", err
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		return string(b), err
	}
	must(f.WriteFile("a/b.txt", []byte("hello")))

	// Open。
	if got, err := readAll("a/b.txt"); err != nil || got != "hello" {
		t.Errorf("Open = %q, %v", got, err)
	}
	if _, err := f.Open("nope"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open(無い) = %v", err)
	}
	if _, err := f.Open("a"); err == nil {
		t.Error("dir の Open が通った")
	}

	// AppendFile: 無ければ作り (親の dir も) 足した順に並ぶ。
	must(f.AppendFile("log/x.txt", []byte("1")))
	must(f.AppendFile("log/x.txt", []byte("2")))
	if got, err := f.ReadFile("log/x.txt"); err != nil || string(got) != "12" {
		t.Errorf("AppendFile 2 回の後 = %q, %v", got, err)
	}
	if err := f.AppendFile("a", []byte("x")); err == nil {
		t.Error("dir への AppendFile が通った")
	}

	// MkdirAll: 入れ子を作り 在れば何もしない。途中が file なら 誤り。
	must(f.MkdirAll("m/n/o"))
	must(f.MkdirAll("m/n/o"))
	must(f.MkdirAll("."))
	if info, err := f.Stat("m/n/o"); err != nil || !info.IsDir {
		t.Errorf("MkdirAll の後の Stat = %+v, %v", info, err)
	}
	if err := f.MkdirAll("a/b.txt/x"); err == nil {
		t.Error("file の下への MkdirAll が通った")
	}

	// Lstat: symlink でない物は Stat と同じ。
	if info, err := f.Lstat("a/b.txt"); err != nil || info != (outside.FileInfo{Name: "b.txt", Size: 5, Mode: 0o644}) {
		t.Errorf("Lstat(file) = %+v, %v", info, err)
	}
	if info, err := f.Lstat("m"); err != nil || info != (outside.FileInfo{Name: "m", IsDir: true, Mode: 0o755}) {
		t.Errorf("Lstat(dir) = %+v, %v", info, err)
	}
	if _, err := f.Lstat("nope"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(無い) = %v", err)
	}

	// Rename: file を移す・file を置き換える・dir を中身ごと移す。
	must(f.Rename("a/b.txt", "m/c.txt"))
	if got, err := f.ReadFile("m/c.txt"); err != nil || string(got) != "hello" {
		t.Errorf("Rename の移し先 = %q, %v", got, err)
	}
	if _, err := f.Stat("a/b.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Rename の移す元が残った: %v", err)
	}
	must(f.WriteFile("m/d.txt", []byte("other")))
	must(f.Rename("m/d.txt", "m/c.txt"))
	if got, err := f.ReadFile("m/c.txt"); err != nil || string(got) != "other" {
		t.Errorf("置き換えた Rename の後 = %q, %v", got, err)
	}
	must(f.Rename("m/c.txt", "m/c.txt"))
	must(f.Rename("m", "m2"))
	if got, err := f.ReadFile("m2/c.txt"); err != nil || string(got) != "other" {
		t.Errorf("dir の Rename の後の中身 = %q, %v", got, err)
	}
	if info, err := f.Stat("m2/n/o"); err != nil || !info.IsDir {
		t.Errorf("dir の Rename の後の入れ子 = %+v, %v", info, err)
	}
	if _, err := f.Stat("m"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("dir の Rename の移す元が残った: %v", err)
	}
	if err := f.Rename("nope", "x"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Rename(無い) = %v", err)
	}
	if err := f.Rename("m2/c.txt", "zz/c.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("移し先の親が無い Rename = %v", err)
	}
	if err := f.Rename("m2/c.txt", "log"); err == nil {
		t.Error("dir の上への Rename が通った")
	}
	if err := f.Rename("m2", "m2/n/inner"); err == nil {
		t.Error("dir を自分の下へ移す Rename が通った")
	}
	if err := f.Rename(".", "root2"); err == nil {
		t.Error("根の Rename が通った")
	}
	for _, p := range []string{"..", "../x", abs, ""} {
		if err := f.Rename("m2/c.txt", p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("根の外へ (%q) の Rename = %v", p, err)
		}
		if err := f.Rename(p, "m2/from-outside.txt"); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("根の外から (%q) の Rename = %v", p, err)
		}
	}
	if got, err := f.ReadFile("m2/c.txt"); err != nil || string(got) != "other" {
		t.Errorf("通らなかった Rename の後に 移す元が変わった: %q, %v", got, err)
	}

	// RemoveAll: 中身ごと消え 無い path は 誤りでなく 根は 誤り。
	must(f.RemoveAll("m2"))
	if _, err := f.Stat("m2"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("RemoveAll の後に残った: %v", err)
	}
	must(f.RemoveAll("m2"))
	must(f.RemoveAll("log/x.txt"))
	if err := f.RemoveAll("."); err == nil {
		t.Error("根の RemoveAll が通った")
	}
	if got, err := f.ReadDir("."); err != nil || !reflect.DeepEqual(got, []outside.DirEntry{{Name: "a", IsDir: true}, {Name: "log", IsDir: true}}) {
		t.Errorf("根の RemoveAll の後の一覧 = %+v, %v", got, err)
	}

	// 根の外。
	for _, p := range []string{"..", "../x", "a/../../x", abs, ""} {
		if _, err := f.Open(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("Open(%q) = %v", p, err)
		}
		if err := f.AppendFile(p, []byte("x")); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("AppendFile(%q) = %v", p, err)
		}
		if err := f.MkdirAll(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("MkdirAll(%q) = %v", p, err)
		}
		if _, err := f.Lstat(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("Lstat(%q) = %v", p, err)
		}
		if err := f.RemoveAll(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("RemoveAll(%q) = %v", p, err)
		}
	}
}

func TestContract_TC_OUT_FS_MORE(t *testing.T) {
	newAbs := func(t *testing.T) string {
		abs := filepath.Join(testDir(t), "outside.txt")
		if err := os.WriteFile(abs, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		return abs
	}
	checkAbs := func(t *testing.T, abs string) {
		if data, err := os.ReadFile(abs); err != nil || string(data) != "keep" {
			t.Errorf("根の外の file が変わった: %q, %v", data, err)
		}
	}
	t.Run("real", func(t *testing.T) {
		root := testDir(t)
		f, err := outside.NewFS(root)
		if err != nil {
			t.Fatal(err)
		}
		abs := newAbs(t)
		fsMoreContract(t, f, abs)
		checkAbs(t, abs)
		if _, err := os.Stat(root); err != nil {
			t.Errorf("根が消えた: %v", err)
		}
	})
	t.Run("mock", func(t *testing.T) {
		f, err := outsidemock.NewFS(nil)
		if err != nil {
			t.Fatal(err)
		}
		abs := newAbs(t)
		fsMoreContract(t, f, abs)
		checkAbs(t, abs)
	})
}

// TC-OUT-FS-MORE (real の symlink): Lstat は symlink を辿らず IsSymlink で名乗る。Stat は辿る。
// 根の外を指す symlink は Lstat で名乗れるが 辿る操作 (Open・Stat) は止まり RemoveAll は 指す先を消さない。
func TestRealFS_TC_OUT_FS_MORE_Symlink(t *testing.T) {
	root := testDir(t)
	secret := filepath.Join(testDir(t), "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	// umask に依らず 権限を決める (Stat の Mode を比べるため)。
	if err := os.Chmod(filepath.Join(root, "target.txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", filepath.Join(root, "in")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "out")); err != nil {
		t.Fatal(err)
	}
	f, err := outside.NewFS(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"in", "out"} {
		if info, err := f.Lstat(name); err != nil || info != (outside.FileInfo{Name: name, IsSymlink: true}) {
			t.Errorf("Lstat(%s) = %+v, %v", name, info, err)
		}
	}
	if info, err := f.Stat("in"); err != nil || info != (outside.FileInfo{Name: "in", Size: 3, Mode: 0o644}) {
		t.Errorf("Stat(in) = %+v, %v (辿った先を返す)", info, err)
	}
	if rc, err := f.Open("out"); err == nil {
		rc.Close()
		t.Error("symlink 越しに 根の外が開けた")
	}
	if _, err := f.Stat("out"); err == nil {
		t.Error("symlink 越しに 根の外の Stat が通った")
	}
	if err := f.AppendFile("out", []byte("x")); err == nil {
		t.Error("symlink 越しに 根の外へ足せた")
	}
	if err := f.RemoveAll("out"); err != nil {
		t.Errorf("根の外を指す symlink の RemoveAll = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "out")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("RemoveAll の後に symlink が残った: %v", err)
	}
	if data, err := os.ReadFile(secret); err != nil || string(data) != "secret" {
		t.Errorf("根の外の file が変わった: %q, %v", data, err)
	}
}

// TC-OUT-PROC-LOOKPATH: 宣言した program の path を返し 宣言に無い名前は ErrUndeclaredProgram。
// 宣言に在っても 実行 file が無ければ exec.ErrNotFound の仲間。
func TestProc_TC_OUT_PROC_LOOKPATH(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		dir := testDir(t)
		tool := filepath.Join(dir, "tool")
		onPath := filepath.Join(dir, "onpath")
		for _, p := range []string{tool, onPath} {
			if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		// PATH は test が作った dir だけ (機体の PATH を読まない)。
		t.Setenv("PATH", dir)
		p := outside.NewProc(map[string]string{
			"tool":    tool,
			"onpath":  "",
			"absent":  "",
			"missing": filepath.Join(dir, "nope"),
		})
		if got, err := p.LookPath("tool"); err != nil || got != tool {
			t.Errorf("LookPath(tool) = %q, %v (want %q)", got, err, tool)
		}
		if got, err := p.LookPath("onpath"); err != nil || got != onPath {
			t.Errorf("LookPath(onpath) = %q, %v (want %q)", got, err, onPath)
		}
		for _, name := range []string{"absent", "missing"} {
			if got, err := p.LookPath(name); !errors.Is(err, exec.ErrNotFound) {
				t.Errorf("LookPath(%s) = %q, %v (want exec.ErrNotFound)", name, got, err)
			}
		}
		if got, err := p.LookPath("undeclared"); !errors.Is(err, outside.ErrUndeclaredProgram) {
			t.Errorf("宣言に無い LookPath = %q, %v", got, err)
		}
	})
	t.Run("mock", func(t *testing.T) {
		p := outsidemock.NewProc(map[string]func(outside.Cmd) outside.Result{"tool": nil, "absent": nil}).
			WithPaths(map[string]string{"tool": "/opt/bin/tool"})
		if got, err := p.LookPath("tool"); err != nil || got != "/opt/bin/tool" {
			t.Errorf("LookPath(tool) = %q, %v", got, err)
		}
		if got, err := p.LookPath("absent"); !errors.Is(err, exec.ErrNotFound) {
			t.Errorf("path を渡していない LookPath = %q, %v", got, err)
		}
		if got, err := p.LookPath("undeclared"); !errors.Is(err, outside.ErrUndeclaredProgram) {
			t.Errorf("宣言に無い LookPath = %q, %v", got, err)
		}
		want := []outsidemock.Call{
			{Method: "LookPath", Args: []string{"tool"}},
			{Method: "LookPath", Args: []string{"absent"}},
			{Method: "LookPath", Args: []string{"undeclared"}},
		}
		if got := p.Calls(); !reflect.DeepEqual(got, want) {
			t.Errorf("記録 = %+v", got)
		}
	})
}

// TC-OUT-PROC-UNSET (real): Unset の名は 親に在っても 子へ渡らない。Env で同じ名を足せば Env が効く。
// 触る環境変数は この test が t.Setenv で設けた物だけ。
func TestRealProc_TC_OUT_PROC_UNSET(t *testing.T) {
	const name = "OUTSIDE_TEST_PROC_UNSET"
	t.Setenv(name, "parent")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := outside.NewProc(map[string]string{"helper": self})
	run := func(c outside.Cmd) string {
		t.Helper()
		c.Name, c.Args = "helper", []string{"outside-helper", "getenv", name}
		res, err := p.Run(context.Background(), c)
		if err != nil || res.ExitCode != 0 {
			t.Fatalf("Run = %+v, %v", res, err)
		}
		return string(res.Stdout)
	}
	for _, tc := range []struct {
		label string
		cmd   outside.Cmd
		want  string
	}{
		{"Unset なし", outside.Cmd{}, "set:parent"},
		{"Unset あり", outside.Cmd{Unset: []string{name}}, "unset"},
		{"Unset と Env に同じ名", outside.Cmd{Unset: []string{name}, Env: []string{name + "=child"}}, "set:child"},
		{"別の名の Unset", outside.Cmd{Unset: []string{name + "_OTHER"}}, "set:parent"},
	} {
		if got := run(tc.cmd); got != tc.want {
			t.Errorf("%s = %q (want %q)", tc.label, got, tc.want)
		}
	}
}

// TC-OUT-PROC-UNSET (mock): Cmds() に Unset が そのまま残る。
func TestMockProc_TC_OUT_PROC_UNSET(t *testing.T) {
	p := outsidemock.NewProc(map[string]func(outside.Cmd) outside.Result{"x": nil})
	if _, err := p.Run(context.Background(), outside.Cmd{Name: "x", Unset: []string{"A", "B"}}); err != nil {
		t.Fatal(err)
	}
	if got := p.Cmds()[0].Unset; !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Errorf("記録の Unset = %q", got)
	}
}
