package outside_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// TC-OUT-CONTRACT: 口ごとの契約の test を real と mock の両方に掛ける。
// real に掛ける時は test が自分で作った物 (t.TempDir の下・自分で起こした子 process) だけを触る。

// TestMain は 引数が outside-helper の時だけ 子 process 役として振る舞う (Proc の契約の相手)。
func TestMain(m *testing.M) {
	if len(os.Args) >= 3 && os.Args[1] == "outside-helper" {
		helperMain(os.Args[2:])
	}
	os.Exit(m.Run())
}

func helperMain(args []string) {
	switch args[0] {
	case "echo":
		fmt.Print(strings.Join(args[1:], " "))
		os.Exit(0)
	case "exit":
		n, _ := strconv.Atoi(args[1])
		os.Exit(n)
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
	if info, err := f.Stat("a/b.txt"); err != nil || info != (outside.FileInfo{Name: "b.txt", Size: 5}) {
		t.Errorf("Stat(file) = %+v, %v", info, err)
	}
	if info, err := f.Stat("a"); err != nil || info != (outside.FileInfo{Name: "a", IsDir: true}) {
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
		abs := filepath.Join(t.TempDir(), "outside.txt")
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
		f, err := outside.NewFS(t.TempDir())
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
	root := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret.txt")
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
	if _, err := outside.NewFS(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("無い根で FS が作れた")
	}
}

// 契約の木: c1 (a.txt, dir/b.txt) → c2 (a.txt を変え c.txt を足す)。head は c2。
func gitContract(t *testing.T, g outside.Git, c1, c2 string) {
	t.Helper()
	if got, err := g.Head(); err != nil || got != c2 {
		t.Errorf("Head = %q, %v (want %q)", got, err, c2)
	}
	if got, err := g.ListFiles(); err != nil || !reflect.DeepEqual(got, []string{"a.txt", "c.txt", "dir/b.txt"}) {
		t.Errorf("ListFiles = %v, %v", got, err)
	}
	if got, err := g.MergeBase(c1, c2); err != nil || got != c1 {
		t.Errorf("MergeBase = %q, %v (want %q)", got, err, c1)
	}
	if got, err := g.DiffNames(c1, c2); err != nil || !reflect.DeepEqual(got, []string{"a.txt", "c.txt"}) {
		t.Errorf("DiffNames = %v, %v", got, err)
	}
	if got, err := g.IsAncestor(c1, c2); err != nil || !got {
		t.Errorf("IsAncestor(c1, c2) = %v, %v", got, err)
	}
	if got, err := g.IsAncestor(c2, c1); err != nil || got {
		t.Errorf("IsAncestor(c2, c1) = %v, %v", got, err)
	}
	if _, err := g.MergeBase(c1, "no-such-commit"); err == nil {
		t.Error("知らない commit の MergeBase が通った")
	}
}

// gitEnv は test の git に渡す環境の全部。機体の設定 (global・system) を読ませない。
var gitEnv = []string{
	"GIT_CONFIG_GLOBAL=" + os.DevNull,
	"GIT_CONFIG_NOSYSTEM=1",
	"GIT_AUTHOR_NAME=outside-test",
	"GIT_AUTHOR_EMAIL=outside-test@example.invalid",
	"GIT_COMMITTER_NAME=outside-test",
	"GIT_COMMITTER_EMAIL=outside-test@example.invalid",
	"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z",
	"GIT_COMMITTER_DATE=2000-01-01T00:00:00Z",
}

// gitDo は dir の repo で git を起こす。環境は gitEnv だけを渡す。
func gitDo(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = gitEnv
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	f, err := outside.NewFS(dir)
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := f.WriteFile(name, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestContract_TC_OUT_CONTRACT_Git(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		// repo は t.TempDir の中に この test が作る。
		dir := t.TempDir()
		gitDo(t, dir, "init", "-q")
		writeFiles(t, dir, map[string]string{"a.txt": "1", "dir/b.txt": "1"})
		gitDo(t, dir, "add", "-A")
		gitDo(t, dir, "commit", "-q", "-m", "c1")
		c1 := gitDo(t, dir, "rev-parse", "HEAD")
		writeFiles(t, dir, map[string]string{"a.txt": "2", "c.txt": "1"})
		gitDo(t, dir, "add", "-A")
		gitDo(t, dir, "commit", "-q", "-m", "c2")
		c2 := gitDo(t, dir, "rev-parse", "HEAD")
		gitContract(t, outside.NewGit(dir, gitEnv[0], gitEnv[1]), c1, c2)
	})
	t.Run("mock", func(t *testing.T) {
		c1, c2 := "1111111", "2222222"
		g := outsidemock.NewGit(outsidemock.GitValues{
			Head:       c2,
			Files:      []string{"a.txt", "c.txt", "dir/b.txt"},
			MergeBase:  map[outsidemock.Pair]string{{A: c1, B: c2}: c1},
			DiffNames:  map[outsidemock.Pair][]string{{A: c1, B: c2}: {"a.txt", "c.txt"}},
			IsAncestor: map[outsidemock.Pair]bool{{A: c1, B: c2}: true, {A: c2, B: c1}: false},
		})
		gitContract(t, g, c1, c2)
	})
}

// real の Git は commit の無い木で head を引けない。
func TestRealGit_NoHeadWithoutCommit(t *testing.T) {
	dir := t.TempDir()
	gitDo(t, dir, "init", "-q")
	if sha, err := outside.NewGit(dir, gitEnv[0], gitEnv[1]).Head(); err == nil {
		t.Errorf("commit の無い木で Head が通った: %q", sha)
	}
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
			}
			return outside.Result{ExitCode: 64}
		}
		procContract(t, outsidemock.NewProc(map[string]func(outside.Cmd) outside.Result{"helper": helper}), prefix)
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
	if err := w.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if w.Path() != "" || exists(p1) || exists(p2) {
		t.Errorf("Cleanup の後に残った: Path=%q %v %v", w.Path(), exists(p1), exists(p2))
	}
}

func TestContract_TC_OUT_CONTRACT_Workdir(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		root := t.TempDir()
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
		parent := t.TempDir()
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
