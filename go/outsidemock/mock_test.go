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

// TC-OUT-CLOCK-TIMER (mock): Advance・Sleep で 期限を過ぎた物を 期限の順に発火し 送る値は その期限の時刻。
// Ticker は 過ぎた回数だけ発火を試み buffer 1 を超えた分は落ちる。実時間を待たない。
func TestClockMock_TC_OUT_CLOCK_TIMER(t *testing.T) {
	start := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
	at := func(d time.Duration) time.Time { return start.Add(d) }
	got := func(ch <-chan time.Time) (time.Time, bool) {
		select {
		case v := <-ch:
			return v, true
		default:
			return time.Time{}, false
		}
	}
	expect := func(ch <-chan time.Time, want time.Time, what string) {
		t.Helper()
		if v, ok := got(ch); !ok || !v.Equal(want) {
			t.Errorf("%s = %v, %v (want %v)", what, v, ok, want)
		}
	}
	empty := func(ch <-chan time.Time, what string) {
		t.Helper()
		if v, ok := got(ch); ok {
			t.Errorf("%s が届いた: %v", what, v)
		}
	}

	c := outsidemock.NewClock(start)
	t2 := c.NewTimer(2 * time.Second)
	t5 := c.NewTimer(5 * time.Second)
	a3 := c.After(3 * time.Second)
	tk := c.NewTicker(time.Second)
	c.Advance(4 * time.Second)
	expect(t2.C(), at(2*time.Second), "2 秒の Timer")
	expect(a3, at(3*time.Second), "3 秒の After")
	empty(t5.C(), "5 秒の Timer (期限前)")
	// Ticker は 1・2・3・4 秒の 4 回を試み 受け手が読まないので 2 回目からは落ちた。
	expect(tk.C(), at(time.Second), "Ticker の最初")
	empty(tk.C(), "落ちた Ticker の発火")
	if now := c.Now(); !now.Equal(at(4 * time.Second)) {
		t.Errorf("Advance の後の Now = %v", now)
	}
	// Sleep でも発火する。Ticker の次は 5 秒。
	c.Sleep(time.Second)
	expect(t5.C(), at(5*time.Second), "Sleep で期限を過ぎた Timer")
	expect(tk.C(), at(5*time.Second), "Ticker の 5 秒")
	// 期限ちょうどまで進めると 発火する (期限を過ぎた = 期限の時刻 以上)。
	c.Advance(time.Second)
	expect(tk.C(), at(6*time.Second), "Ticker の 6 秒")

	// Stop・Reset の戻りは time と同じ意味。
	if t2.Stop() {
		t.Error("発火済みの Stop が true")
	}
	t9 := c.NewTimer(3 * time.Second) // 期限 9 秒
	if !t9.Reset(time.Second) {       // 期限 7 秒へ
		t.Error("動いている Timer の Reset が false")
	}
	c.Advance(time.Second)
	expect(t9.C(), at(7*time.Second), "Reset した Timer")
	c.Advance(5 * time.Second)
	empty(t9.C(), "Reset 前の期限")
	if t2.Reset(time.Second) {
		t.Error("済んだ Timer の Reset が true")
	}
	if !t2.Stop() {
		t.Error("Reset で掛け直した Timer の Stop が false")
	}
	c.Advance(time.Hour)
	empty(t2.C(), "Stop した Timer")
	// 0 以下の期限は 掛けた時に 今の時刻を送る。
	now := c.Now()
	expect(c.NewTimer(0).C(), now, "0 の Timer")
	expect(c.After(-time.Second), now, "負の After")
	tk.Stop()
	c.Advance(time.Hour)
	empty(tk.C(), "Stop した Ticker")

	// 1 年を 1 ミリ秒の Ticker で進めても すぐ戻る (落ちる回は 数えて飛ばす)。
	fast := c.NewTicker(time.Millisecond)
	began := time.Now()
	c.Advance(365 * 24 * time.Hour)
	if took := time.Since(began); took > 5*time.Second {
		t.Errorf("1 年の Advance に 実時間 %v かかった", took)
	}
	base := now.Add(time.Hour)
	expect(fast.C(), base.Add(time.Millisecond), "速い Ticker の最初")
	c.Advance(time.Millisecond)
	expect(fast.C(), base.Add(365*24*time.Hour+time.Millisecond), "速い Ticker の 1 年後の次")
	fast.Stop()

	for method, n := range map[string]int{"NewTimer": 4, "After": 2, "NewTicker": 2, "Timer.Stop": 2, "Timer.Reset": 2, "Ticker.Stop": 2} {
		if calls := outsidemock.CallsOf(c.Calls(), method); len(calls) != n {
			t.Errorf("%s の記録 = %d 件 (want %d): %+v", method, len(calls), n, calls)
		}
	}
	if calls := outsidemock.CallsOf(c.Calls(), "NewTimer"); calls[0].Args[0] != "2s" {
		t.Errorf("NewTimer の記録の引数 = %+v", calls[0])
	}
}

// 並行に安全: 別の goroutine が待つ channel に Advance が届き 掛ける・止める・進めるが 重なっても壊れない。
func TestClockMock_TimerConcurrent(t *testing.T) {
	c := outsidemock.NewClock(time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC))
	const n = 50
	chans := make([]<-chan time.Time, n)
	for i := range chans {
		chans[i] = c.After(time.Duration(i+1) * time.Second)
	}
	var wg sync.WaitGroup
	results := make(chan time.Time, n)
	for _, ch := range chans {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- <-ch
		}()
	}
	var churn sync.WaitGroup
	for range 8 {
		churn.Add(1)
		go func() {
			defer churn.Done()
			for range 50 {
				tm := c.NewTimer(time.Second)
				tk := c.NewTicker(time.Second)
				tm.Reset(time.Minute)
				tm.Stop()
				tk.Stop()
				c.Sleep(time.Millisecond)
			}
		}()
	}
	c.Advance(time.Duration(n) * time.Second)
	churn.Wait()
	wg.Wait()
	close(results)
	if got := len(results); got != n {
		t.Errorf("届いた数 = %d (want %d)", got, n)
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
