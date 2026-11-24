package outside_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// mockWorld は RunToolWith に渡す mock の環境。置き場は work だけが在る。
type mockWorld struct {
	env      *outsidemock.Env
	fs       *outsidemock.FS
	worktree string // 空なら git の木の外
	proc     *outsidemock.Proc
	workdirs []*outsidemock.Workdir
}

// worldTree は mock の環境の木。data と other と 空の work を持つ。
var worldTree = outsidemock.Tree{
	"data/in.txt":  "in",
	"other/no.txt": "no",
	"work/":        "",
}

func newWorld(t *testing.T, worktree string) *mockWorld {
	t.Helper()
	fsys, err := outsidemock.NewFS(worldTree)
	if err != nil {
		t.Fatal(err)
	}
	return &mockWorld{
		env:      outsidemock.NewEnv(map[string]string{"ALLOWED": "yes", "OTHER": "hidden", "WORK_ROOT": "work", "DATA_ROOT": ".", "MISSING_ROOT": "nowork"}),
		fs:       fsys,
		worktree: worktree,
		proc: outsidemock.NewProc(map[string]func(outside.Cmd) outside.Result{
			"toolx": outsidemock.Fixed(outside.Result{Stdout: []byte("toolx version 2.9.1\n")}),
			"other": nil,
		}),
	}
}

func (w *mockWorld) base() outside.Base {
	return outside.Base{
		Clock:   outsidemock.NewClock(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
		Env:     w.env,
		FS:      w.fs,
		Proc:    w.proc,
		SysInfo: outsidemock.NewSysInfo(outsidemock.SysInfoValues{Hostname: "host-a", Pid: 7}),
		IdGen:   outsidemock.NewIdGen(),
		Worktree: func() (string, error) {
			if w.worktree == "" {
				return "", errors.New("mock の世界: .git が無い")
			}
			return w.worktree, nil
		},
		Workdir: func(root string) (outside.Workdir, error) {
			wd, err := outsidemock.NewWorkdir(root, root == "work")
			if err != nil {
				return nil, err
			}
			w.workdirs = append(w.workdirs, wd)
			return wd, nil
		},
		EnsureWorkdir: func(root string) (outside.Workdir, error) {
			wd, err := outsidemock.NewWorkdir(root, true)
			if err != nil {
				return nil, err
			}
			w.workdirs = append(w.workdirs, wd)
			return wd, nil
		},
	}
}

// TC-OUT-RUNTOOL-MISSING: 宣言に合わない環境で 足りない物を名乗って終わり 処理を呼ばない。
func TestRunTool_TC_OUT_RUNTOOL_MISSING(t *testing.T) {
	cases := []struct {
		name     string
		decl     outside.Decl
		worktree string
		want     []string // 誤りの文に在るべき語
	}{
		{
			name: "git の木が無く program の版が足りない",
			decl: outside.Decl{RequiresGitWorktree: true, Programs: map[string]string{"toolx": "3"}},
			want: []string{"git の木", "toolx", "3"},
		},
		{
			name:     "program が在らない",
			decl:     outside.Decl{Programs: map[string]string{"tooly": ""}},
			worktree: "repo",
			want:     []string{"tooly"},
		},
		{
			name:     "版の桁が違う (2.9 は 2.90 ではない)",
			decl:     outside.Decl{Programs: map[string]string{"toolx": "2.90"}},
			worktree: "repo",
			want:     []string{"toolx", "2.90"},
		},
		{
			name:     "環境変数が指す 作業場の置き場が無い",
			decl:     outside.Decl{WorkdirRoot: "$MISSING_ROOT"},
			worktree: "repo",
			want:     []string{"nowork"},
		},
		{
			name:     "置き場を指す環境変数が無い",
			decl:     outside.Decl{WorkdirRoot: "$NO_SUCH_ROOT"},
			worktree: "repo",
			want:     []string{"NO_SUCH_ROOT"},
		},
		{
			name:     "環境変数の下の外へ出る置き場",
			decl:     outside.Decl{WorkdirRoot: "$WORK_ROOT/../x"},
			worktree: "repo",
			want:     []string{"WORK_ROOT/../x"},
		},
		{
			name:     "fs_roots の dir が無い",
			decl:     outside.Decl{FSRoots: []string{"nodata"}},
			worktree: "repo",
			want:     []string{"nodata"},
		},
		{
			name:     "fs_roots が根の外",
			decl:     outside.Decl{FSRoots: []string{"../up"}},
			worktree: "repo",
			want:     []string{"../up"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, tc.worktree)
			called := 0
			err := outside.RunToolWith(tc.decl, w.base(), func(outside.Set) error {
				called++
				return nil
			})
			if called != 0 {
				t.Errorf("処理が %d 回 呼ばれた", called)
			}
			var mm *outside.MismatchError
			if !errors.As(err, &mm) {
				t.Fatalf("MismatchError ではない: %v", err)
			}
			for _, word := range tc.want {
				if !strings.Contains(err.Error(), word) {
					t.Errorf("誤りの文 %q に %q が無い", err.Error(), word)
				}
			}
			if code := outside.ExitCode(err); code != outside.ExitUndetermined {
				t.Errorf("exit の番号 = %d (want %d)", code, outside.ExitUndetermined)
			}
		})
	}
}

// TC-OUT-RUNTOOL-OK: 宣言に合う環境で 宣言の範囲に絞った set を渡して 処理を 1 回 呼ぶ。
func TestRunTool_TC_OUT_RUNTOOL_OK(t *testing.T) {
	for _, workdirRoot := range []string{"work", "$WORK_ROOT", "$DATA_ROOT/work"} {
		t.Run(workdirRoot, func(t *testing.T) {
			w := newWorld(t, "repo")
			decl := outside.Decl{
				RequiresGitWorktree: true,
				Programs:            map[string]string{"toolx": "2.9"},
				WorkdirRoot:         workdirRoot,
				Env:                 []string{"ALLOWED"},
				FSRoots:             []string{"data"},
			}
			called := 0
			err := outside.RunToolWith(decl, w.base(), func(s outside.Set) error {
				called++
				ctx := context.Background()
				// Env: 宣言した名前だけが読める。
				if v, ok := s.Env.Lookup("ALLOWED"); !ok || v != "yes" {
					t.Errorf("Lookup(ALLOWED) = %q, %v", v, ok)
				}
				if v, ok := s.Env.Lookup("OTHER"); ok {
					t.Errorf("宣言に無い環境変数が読めた: %q", v)
				}
				// FS: 宣言した根の下だけ。
				if data, err := s.FS.ReadFile("data/in.txt"); err != nil || string(data) != "in" {
					t.Errorf("ReadFile(data/in.txt) = %q, %v", data, err)
				}
				if _, err := s.FS.ReadFile("other/no.txt"); !errors.Is(err, outside.ErrOutsideRoot) {
					t.Errorf("fs_roots の外が読めた: %v", err)
				}
				if err := s.FS.WriteFile("other/new.txt", nil); !errors.Is(err, outside.ErrOutsideRoot) {
					t.Errorf("fs_roots の外へ書けた: %v", err)
				}
				if _, err := s.FS.ReadDir("."); !errors.Is(err, outside.ErrOutsideRoot) {
					t.Errorf("根の一覧が引けた: %v", err)
				}
				// Proc: 宣言した program だけ。環境に在っても 宣言に無ければ 誤り。
				if _, err := s.Proc.Run(ctx, outside.Cmd{Name: "toolx"}); err != nil {
					t.Errorf("Run(toolx) = %v", err)
				}
				if _, err := s.Proc.Run(ctx, outside.Cmd{Name: "other"}); !errors.Is(err, outside.ErrUndeclaredProgram) {
					t.Errorf("宣言に無い program が起きた: %v", err)
				}
				// SysInfo・IdGen は base の物がそのまま渡る。Workdir。
				if h, err := s.SysInfo.Hostname(); err != nil || h != "host-a" {
					t.Errorf("Hostname = %q, %v", h, err)
				}
				if id := s.IdGen.NewID(); id != "id-0001" {
					t.Errorf("NewID = %q", id)
				}
				if p, err := s.Workdir.New("job"); err != nil || p != "work/job" {
					t.Errorf("Workdir.New = %q, %v", p, err)
				}
				if s.Clock == nil {
					t.Error("Clock が渡されていない")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if called != 1 {
				t.Errorf("処理が %d 回 呼ばれた", called)
			}
			if code := outside.ExitCode(err); code != outside.ExitOK {
				t.Errorf("exit の番号 = %d", code)
			}
			// 終わりに 作業場が消えている。
			if len(w.workdirs) != 1 || len(w.workdirs[0].Live()) != 0 {
				t.Errorf("作業場が残った: %+v", w.workdirs)
			}
			// 宣言に無い program は mock の環境まで届いていない。
			for _, c := range w.proc.Calls() {
				if c.Args[0] == "other" {
					t.Errorf("宣言に無い program が環境へ届いた: %+v", c)
				}
			}
		})
	}
}

// 置き場を宣言しない時 作業場は作れない。処理の誤りは ツールの否 (判定不能とは別の番号)。
func TestRunTool_NoWorkdirAndToolFailure(t *testing.T) {
	w := newWorld(t, "repo")
	failed := errors.New("ツールの否")
	err := outside.RunToolWith(outside.Decl{}, w.base(), func(s outside.Set) error {
		if _, err := s.Workdir.New("job"); !errors.Is(err, outside.ErrNoWorkdirRoot) {
			t.Errorf("置き場の宣言が無いのに 作業場が作れた: %v", err)
		}
		return failed
	})
	if !errors.Is(err, failed) {
		t.Fatalf("処理の誤りが返らない: %v", err)
	}
	if code := outside.ExitCode(err); code != outside.ExitFail {
		t.Errorf("exit の番号 = %d (want %d)", code, outside.ExitFail)
	}
}

// TC-OUT-SECRET-NOCOPY: secret を読む口 (Env) は 値を返すだけで FS と Workdir に 書き込みを残さない。
func TestSecret_TC_OUT_SECRET_NOCOPY(t *testing.T) {
	w := newWorld(t, "repo")
	w.env = outsidemock.NewEnv(map[string]string{"API_TOKEN": "s3cret"})
	decl := outside.Decl{WorkdirRoot: "work", Env: []string{"API_TOKEN"}, FSRoots: []string{"data"}}
	err := outside.RunToolWith(decl, w.base(), func(s outside.Set) error {
		if v, ok := s.Env.Lookup("API_TOKEN"); !ok || v != "s3cret" {
			t.Errorf("secret の値が返らない: %q, %v", v, ok)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if writes := outsidemock.CallsOf(w.fs.Calls(), "WriteFile"); len(writes) != 0 {
		t.Errorf("FS に書き込みが記録された: %+v", writes)
	}
	for _, wd := range w.workdirs {
		if made := outsidemock.CallsOf(wd.Calls(), "New"); len(made) != 0 {
			t.Errorf("Workdir に作成が記録された: %+v", made)
		}
	}
}

const declTOML = `
requires_git_worktree = true
workdir_root = "work"
env = ["ALLOWED"]
fs_roots = ["data", "docs"]

[programs]
toolz = "2"
toolx = ""
`

// 宣言の TOML は 設計書の項目のとおりに読める。知らない項目と壊れた TOML は 判定不能。
func TestParseDecl(t *testing.T) {
	got, err := outside.ParseDecl([]byte(declTOML))
	if err != nil {
		t.Fatal(err)
	}
	want := outside.Decl{
		RequiresGitWorktree: true,
		Programs:            map[string]string{"toolz": "2", "toolx": ""},
		WorkdirRoot:         "work",
		Env:                 []string{"ALLOWED"},
		FSRoots:             []string{"data", "docs"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseDecl = %+v", got)
	}
	for _, bad := range []string{"unknown_key = 1\n", "fs_roots = [\n"} {
		if _, err := outside.ParseDecl([]byte(bad)); !errors.Is(err, outside.ErrUndetermined) {
			t.Errorf("ParseDecl(%q) = %v", bad, err)
		}
	}
}

// writeTool は testDir の下に ツールの dir (outside.toml・data・work) を作る。
func writeTool(t *testing.T, decl string) (dir, declPath string) {
	t.Helper()
	dir = testDir(t)
	for _, d := range []string{"data", "work"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	declPath = filepath.Join(dir, "outside.toml")
	if err := os.WriteFile(declPath, []byte(decl), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, declPath
}

// real の RunTool は 宣言の file を読み 宣言の dir を基準に real の set を渡す。
func TestRunToolReal_OK(t *testing.T) {
	dir, declPath := writeTool(t, "workdir_root = \"work\"\nfs_roots = [\"data\"]\n")
	called := 0
	var job string
	err := outside.RunTool(declPath, func(s outside.Set) error {
		called++
		if err := s.FS.WriteFile("data/out.txt", []byte("out")); err != nil {
			t.Errorf("WriteFile = %v", err)
		}
		if err := s.FS.WriteFile("work/x.txt", nil); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("fs_roots の外へ書けた: %v", err)
		}
		p, err := s.Workdir.New("job")
		if err != nil {
			t.Errorf("Workdir.New = %v", err)
		}
		job = p
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			t.Errorf("作業場が無い: %v", err)
		}
		return nil
	})
	if err != nil || called != 1 {
		t.Fatalf("RunTool = %v, 呼ばれた回数 %d", err, called)
	}
	if rel, err := filepath.Rel(filepath.Join(dir, "work"), job); err != nil || rel != "job" {
		t.Errorf("作業場 %q が 置き場の直下でない", job)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "data", "out.txt")); err != nil || string(data) != "out" {
		t.Errorf("書いた file = %q, %v", data, err)
	}
	if _, err := os.Stat(job); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("終わりに 作業場が消えていない: %v", err)
	}
}

// real の RunTool は 宣言が無い・読めない・環境が合わない時 処理を呼ばず 判定不能で終わる。
func TestRunToolReal_Undetermined(t *testing.T) {
	// 環境変数が指す置き場は 機体が用意する物 = 無ければ作らず 判定不能 (test が作った dir の下の 在らない path を指させる)。
	missingDir, missingRoot := writeTool(t, "fs_roots = [\"nodata\"]\nworkdir_root = \"$OUTSIDE_TEST_WORK_ROOT\"\n")
	t.Setenv("OUTSIDE_TEST_WORK_ROOT", filepath.Join(missingDir, "nowork"))
	_, unknownKey := writeTool(t, "no_such_field = true\n")
	noFile := filepath.Join(testDir(t), "outside.toml")
	for name, declPath := range map[string]string{"dir が無い": missingRoot, "知らない項目": unknownKey, "宣言が無い": noFile} {
		t.Run(name, func(t *testing.T) {
			called := 0
			err := outside.RunTool(declPath, func(outside.Set) error {
				called++
				return nil
			})
			if called != 0 {
				t.Errorf("処理が %d 回 呼ばれた", called)
			}
			if code := outside.ExitCode(err); code != outside.ExitUndetermined {
				t.Errorf("exit の番号 = %d, err = %v", code, err)
			}
		})
	}
	// 足りない物の名前が 誤りの文に在る。
	err := outside.RunTool(missingRoot, func(outside.Set) error { return nil })
	for _, word := range []string{"nodata", "nowork"} {
		if err == nil || !strings.Contains(err.Error(), word) {
			t.Errorf("誤りの文 %v に %q が無い", err, word)
		}
	}
}

// TC-OUT-WORKDIR-REPO-ENSURE: repo の中の相対 path の置き場は 無ければ作る (新しい clone でも回る)。
// 作った置き場は 宣言の dir の下にあり、処理は 1 回 呼ばれる。
func TestRunToolReal_TC_OUT_WORKDIR_REPO_ENSURE(t *testing.T) {
	dir, declPath := writeTool(t, "workdir_root = \".work/tool\"\n")
	if _, err := os.Stat(filepath.Join(dir, ".work")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("前提: 置き場が既に在る: %v", err)
	}
	called := 0
	err := outside.RunTool(declPath, func(s outside.Set) error {
		called++
		p, err := s.Workdir.New("job")
		if err != nil {
			t.Errorf("Workdir.New = %v", err)
		}
		if rel, err := filepath.Rel(filepath.Join(dir, ".work", "tool"), p); err != nil || rel != "job" {
			t.Errorf("作業場 %q が 置き場の直下でない", p)
		}
		return nil
	})
	if err != nil || called != 1 {
		t.Fatalf("RunTool = %v, 呼ばれた回数 %d", err, called)
	}
	if info, err := os.Stat(filepath.Join(dir, ".work", "tool")); err != nil || !info.IsDir() {
		t.Errorf("置き場が作られていない: %v", err)
	}
}

// TC-OUT-WORKDIR-REPO-ENSURE (mock): mock の base でも repo の中の相対 path は 在らなくても通り $NAME の置き場は 無ければ判定不能。
func TestRunTool_TC_OUT_WORKDIR_REPO_ENSURE_Mock(t *testing.T) {
	w := newWorld(t, "repo")
	called := 0
	if err := outside.RunToolWith(outside.Decl{WorkdirRoot: "newwork"}, w.base(), func(s outside.Set) error {
		called++
		if p, err := s.Workdir.New("job"); err != nil || p != "newwork/job" {
			t.Errorf("Workdir.New = %q, %v", p, err)
		}
		return nil
	}); err != nil || called != 1 {
		t.Fatalf("RunToolWith = %v, 呼ばれた回数 %d", err, called)
	}
	err := outside.RunToolWith(outside.Decl{WorkdirRoot: "$MISSING_ROOT"}, w.base(), func(outside.Set) error { return nil })
	if code := outside.ExitCode(err); code != outside.ExitUndetermined {
		t.Errorf("$NAME の置き場が無いのに exit %d (want %d)", code, outside.ExitUndetermined)
	}
}
