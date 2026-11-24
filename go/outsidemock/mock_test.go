package outsidemock_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"
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
	abs := filepath.Join(testDir(t), "a.txt")
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
	abs := filepath.Join(testDir(t), "a.txt")
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

// TC-OUT-SYSINFO-MOCK: 渡した値だけを返す。渡していない Hostname・HomeDir は 誤り、Pid は 0、IsTTY は false。
func TestSysInfoMock_TC_OUT_SYSINFO_MOCK(t *testing.T) {
	s := outsidemock.NewSysInfo(outsidemock.SysInfoValues{Hostname: "host-a", HomeDir: "/home/a", Pid: 4242, IsTTY: true})
	if got, err := s.Hostname(); err != nil || got != "host-a" {
		t.Errorf("Hostname = %q, %v", got, err)
	}
	if got, err := s.HomeDir(); err != nil || got != "/home/a" {
		t.Errorf("HomeDir = %q, %v", got, err)
	}
	if got := s.Pid(); got != 4242 {
		t.Errorf("Pid = %d", got)
	}
	if !s.IsTTY() {
		t.Error("IsTTY = false (want true)")
	}
	want := []outsidemock.Call{{Method: "Hostname"}, {Method: "HomeDir"}, {Method: "Pid"}, {Method: "IsTTY"}}
	if got := s.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("記録 = %+v", got)
	}

	// 何も渡さない mock は 機体の値を返さない。
	empty := outsidemock.NewSysInfo(outsidemock.SysInfoValues{})
	if got, err := empty.Hostname(); err == nil {
		t.Errorf("渡していない Hostname が通った: %q", got)
	}
	if got, err := empty.HomeDir(); err == nil {
		t.Errorf("渡していない HomeDir が通った: %q", got)
	}
	if got := empty.Pid(); got != 0 {
		t.Errorf("渡していない Pid = %d (want 0)", got)
	}
	if empty.IsTTY() {
		t.Error("渡していない IsTTY = true (want false)")
	}
}

// TC-OUT-IDGEN-MOCK: id-0001 から 呼んだ順に決まった ID を返す。mock ごとに 1 から始まる。並行に呼んでも安全。
func TestIdGenMock_TC_OUT_IDGEN_MOCK(t *testing.T) {
	first := outsidemock.NewIdGen()
	var got []string
	for range 3 {
		got = append(got, first.NewID())
	}
	if want := []string{"id-0001", "id-0002", "id-0003"}; !reflect.DeepEqual(got, want) {
		t.Errorf("1 つ目の mock = %v (want %v)", got, want)
	}
	second := outsidemock.NewIdGen()
	if id := second.NewID(); id != "id-0001" {
		t.Errorf("2 つ目の mock の最初 = %q (want id-0001)", id)
	}
	if calls := first.Calls(); len(calls) != 3 || calls[2].Args[0] != "id-0003" {
		t.Errorf("記録 = %+v", calls)
	}

	// 並行に呼んでも 抜けも重なりも無い。
	const n = 200
	g := outsidemock.NewIdGen()
	ids := make(chan string, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids <- g.NewID()
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		seen[id] = true
	}
	for i := 1; i <= n; i++ {
		if id := fmt.Sprintf("id-%04d", i); !seen[id] {
			t.Errorf("並行に呼んだ ID に %s が無い (%d 個)", id, len(seen))
		}
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

// testDir は この repo の .work/ の下に この test の dir を作り 終わりに消す (system の一時置き場を使わない)。
func testDir(t *testing.T) string {
	t.Helper()
	base, err := filepath.Abs(filepath.Join("..", "..", ".work"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "go-outsidemock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
