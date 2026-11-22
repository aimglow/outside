package outsidemock_test

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// TC-OUT-CLOCK-MOCK: 決めた時刻を返し Advance した分だけ進む。Sleep は 実時間を待たない。
func TestClockMock_TC_OUT_CLOCK_MOCK(t *testing.T) {
	start := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
	c := outsidemock.NewClock(start)
	if got := c.Now(); !got.Equal(start) {
		t.Errorf("Now = %v (want %v)", got, start)
	}
	c.Advance(90 * time.Minute)
	if got, want := c.Now(), start.Add(90*time.Minute); !got.Equal(want) {
		t.Errorf("Advance の後の Now = %v (want %v)", got, want)
	}
	// 1 年の Sleep が すぐ戻る = 実時間を待っていない。
	year := 365 * 24 * time.Hour
	c.Sleep(year)
	if got, want := c.Now(), start.Add(90*time.Minute+year); !got.Equal(want) {
		t.Errorf("Sleep の後の Now = %v (want %v)", got, want)
	}
	c.Sleep(-time.Hour)
	c.Advance(-time.Hour)
	if got, want := c.Now(), start.Add(90*time.Minute+year); !got.Equal(want) {
		t.Errorf("負の長さで 時刻が動いた: %v", got)
	}
	if sleeps := outsidemock.CallsOf(c.Calls(), "Sleep"); len(sleeps) != 2 || sleeps[0].Args[0] != year.String() {
		t.Errorf("Sleep の記録 = %+v", sleeps)
	}
}

// TC-OUT-ENV-MOCK: 渡した表の値だけを返す。機体の環境変数は 結果に現れない。
func TestEnvMock_TC_OUT_ENV_MOCK(t *testing.T) {
	// 機体の側に在る変数 (test が自分で置く) が mockから見えない事を見る。
	t.Setenv("OUTSIDE_MOCK_MACHINE_ONLY", "machine")
	table := map[string]string{"IN_TABLE": "v", "EMPTY": ""}
	e := outsidemock.NewEnv(table)
	table["ADDED_LATER"] = "x" // 渡した後の書き換えは 効かない。
	if v, ok := e.Lookup("IN_TABLE"); !ok || v != "v" {
		t.Errorf("Lookup(IN_TABLE) = %q, %v", v, ok)
	}
	if v, ok := e.Lookup("EMPTY"); !ok || v != "" {
		t.Errorf("Lookup(EMPTY) = %q, %v", v, ok)
	}
	for _, name := range []string{"NOT_IN_TABLE", "OUTSIDE_MOCK_MACHINE_ONLY", "ADDED_LATER"} {
		if v, ok := e.Lookup(name); ok {
			t.Errorf("表に無い %s が返った: %q", name, v)
		}
	}
	if calls := e.Calls(); len(calls) != 5 || calls[0].Args[0] != "IN_TABLE" {
		t.Errorf("Lookup の記録 = %+v", calls)
	}
}

// fsRootTree は TC-OUT-FS-ROOT の木。
var fsRootTree = outsidemock.Tree{
	"docs/a.txt": "A",
	"empty/":     "",
}

// TC-OUT-FS-ROOT: FS は 宣言した根の外の path を 誤りで返す (読まない)。
func TestFSMock_TC_OUT_FS_ROOT(t *testing.T) {
	f, err := outsidemock.NewFS(fsRootTree)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := f.ReadFile("docs/a.txt"); err != nil || string(data) != "A" {
		t.Errorf("根の中の ReadFile = %q, %v", data, err)
	}
	if got, err := f.ReadDir("."); err != nil ||
		!reflect.DeepEqual(got, []outside.DirEntry{{Name: "docs", IsDir: true}, {Name: "empty", IsDir: true}}) {
		t.Errorf("ReadDir(.) = %+v, %v", got, err)
	}
	if got, err := f.ReadDir("empty"); err != nil || len(got) != 0 {
		t.Errorf("空の dir の ReadDir = %+v, %v", got, err)
	}
	if _, err := f.ReadFile("docs/none.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("無い file = %v", err)
	}
	abs := filepath.Join(t.TempDir(), "a.txt")
	for _, p := range []string{"../docs/a.txt", "docs/../../a.txt", "..", abs} {
		if data, err := f.ReadFile(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("根の外 %q が読めた: %q, %v", p, data, err)
		}
		if err := f.WriteFile(p, []byte("x")); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("根の外 %q へ書けた: %v", p, err)
		}
	}
}

// REQ-OUT-MOCK-TREE: 絶対 path と 根の外へ出る path と 矛盾した木の宣言は 誤り。
func TestFSMock_TreeDeclaration(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "a.txt")
	bad := map[string]outsidemock.Tree{
		"絶対 path":         {abs: "x"},
		"根の外":             {"../a.txt": "x"},
		"途中で根の外":          {"a/../../b.txt": "x"},
		"中身の在る dir":       {"dir/": "x"},
		"file の下に file":   {"a": "x", "a/b": "y"},
		"file と同じ名前の dir": {"a": "x", "a/": ""},
	}
	for name, tree := range bad {
		if _, err := outsidemock.NewFS(tree); err == nil {
			t.Errorf("%s の宣言が通った", name)
		}
	}
}

// TC-OUT-GIT-MOCK: 渡した head・一覧・差分を返す。repo を作らない (file system を触る口を持たない)。
func TestGitMock_TC_OUT_GIT_MOCK(t *testing.T) {
	files := []string{"a.go", "b/c.go"}
	diff := []string{"b/c.go"}
	g := outsidemock.NewGit(outsidemock.GitValues{
		Head:       "h1",
		Files:      files,
		MergeBase:  map[outsidemock.Pair]string{{A: "main", B: "h1"}: "m0"},
		DiffNames:  map[outsidemock.Pair][]string{{A: "m0", B: "h1"}: diff},
		IsAncestor: map[outsidemock.Pair]bool{{A: "m0", B: "h1"}: true},
	})
	if got, err := g.Head(); err != nil || got != "h1" {
		t.Errorf("Head = %q, %v", got, err)
	}
	if got, err := g.ListFiles(); err != nil || !reflect.DeepEqual(got, files) {
		t.Errorf("ListFiles = %v, %v", got, err)
	}
	if got, err := g.MergeBase("main", "h1"); err != nil || got != "m0" {
		t.Errorf("MergeBase = %q, %v", got, err)
	}
	if got, err := g.DiffNames("m0", "h1"); err != nil || !reflect.DeepEqual(got, diff) {
		t.Errorf("DiffNames = %v, %v", got, err)
	}
	if got, err := g.IsAncestor("m0", "h1"); err != nil || !got {
		t.Errorf("IsAncestor = %v, %v", got, err)
	}
	// 宣言していない問いは 誤り (黙って空を返さない)。
	if _, err := g.DiffNames("x", "y"); err == nil {
		t.Error("宣言に無い差分が通った")
	}
	if _, err := g.IsAncestor("x", "y"); err == nil {
		t.Error("宣言に無い祖先の問いが通った")
	}
	if _, err := outsidemock.NewGit(outsidemock.GitValues{}).Head(); err == nil {
		t.Error("head の宣言が無いのに Head が通った")
	}
	want := []outsidemock.Call{
		{Method: "Head"}, {Method: "ListFiles"},
		{Method: "MergeBase", Args: []string{"main", "h1"}},
		{Method: "DiffNames", Args: []string{"m0", "h1"}},
		{Method: "IsAncestor", Args: []string{"m0", "h1"}},
		{Method: "DiffNames", Args: []string{"x", "y"}},
		{Method: "IsAncestor", Args: []string{"x", "y"}},
	}
	if got := g.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("記録 = %+v", got)
	}
}

// TC-OUT-PROC-DECLARED: 宣言に無い program を起こさず 誤りで返す。在る名前は 記録されて結果が返る。
func TestProcMock_TC_OUT_PROC_DECLARED(t *testing.T) {
	ran := 0
	p := outsidemock.NewProc(map[string]func(outside.Cmd) outside.Result{
		"tool": func(c outside.Cmd) outside.Result {
			ran++
			return outside.Result{ExitCode: 7, Stdout: []byte(c.Args[0])}
		},
		"quiet": nil,
	})
	ctx := context.Background()
	res, err := p.Run(ctx, outside.Cmd{Name: "tool", Args: []string{"build", "-v"}})
	if err != nil || res.ExitCode != 7 || string(res.Stdout) != "build" {
		t.Errorf("Run(tool) = %+v, %v", res, err)
	}
	if res, err := p.Run(ctx, outside.Cmd{Name: "quiet"}); err != nil || res.ExitCode != 0 {
		t.Errorf("Run(quiet) = %+v, %v", res, err)
	}
	if _, err := p.Run(ctx, outside.Cmd{Name: "rm", Args: []string{"-rf"}}); !errors.Is(err, outside.ErrUndeclaredProgram) {
		t.Errorf("宣言に無い program = %v", err)
	}
	if ran != 1 {
		t.Errorf("結果の関数が %d 回 呼ばれた", ran)
	}
	want := []outsidemock.Call{
		{Method: "Run", Args: []string{"tool", "build", "-v"}},
		{Method: "Run", Args: []string{"quiet"}},
		{Method: "Run", Args: []string{"rm", "-rf"}},
	}
	if got := p.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("記録 = %+v", got)
	}
}
