package outside_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// TC-OUT-NO-TOOL-NAME: Go の部品は 特定のツールの名前の口を持たず git の program を起こさない。

// toolNames は 部品の公開の名前に現れてはならない ツールの名前。
var toolNames = []string{"Git", "Docker", "Ffmpeg", "FFmpeg"}

// declFields は 設計書の tool_entry.declaration が名前を決める 宣言の項目。口ではない (git の木の中かを問う 宣言の bool)。
var declFields = map[string]bool{"RequiresGitWorktree": true}

// exportedNames は dir の package (test の file を除く) の 公開の名前 (型・関数・変数・定数・method・struct の field・interface の method) を返す。
func exportedNames(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	add := func(id *ast.Ident) {
		if id != nil && id.IsExported() {
			names = append(names, id.Name)
		}
	}
	for _, pkg := range pkgs {
		ast.Inspect(pkg, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				add(x.Name)
			case *ast.TypeSpec:
				add(x.Name)
			case *ast.ValueSpec:
				for _, id := range x.Names {
					add(id)
				}
			case *ast.Field:
				for _, id := range x.Names {
					add(id)
				}
			}
			return true
		})
	}
	return names
}

func TestNoToolName_TC_OUT_NO_TOOL_NAME_公開の名前(t *testing.T) {
	for _, dir := range []string{".", "outsidemock"} {
		names := exportedNames(t, dir)
		if len(names) == 0 {
			t.Fatalf("%s の公開の名前が 1 つも読めない (検査が成り立たない)", dir)
		}
		for _, name := range names {
			if declFields[name] {
				continue
			}
			for _, tool := range toolNames {
				if strings.Contains(name, tool) {
					t.Errorf("%s の公開の名前 %s が ツールの名前 %s を持つ", dir, name, tool)
				}
			}
		}
	}
	// 陽性対照: 検査が ツールの名前の口を捕る事を test が作った dir の source で確かめる。
	fixture := testDir(t)
	src := "package fixture\n\ntype Git interface{ Head() (string, error) }\n"
	if err := os.WriteFile(filepath.Join(fixture, "fixture.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if names := exportedNames(t, fixture); !reflect.DeepEqual(names, []string{"Git", "Head"}) {
		t.Errorf("陽性対照の公開の名前 = %v (want [Git Head])", names)
	}
}

// makeRepo は test の dir の下に 宣言の dir (tools/x/outside.toml) と data/in.txt を持つ木を作り 宣言の path を返す。
// gitEntry が "dir" なら .git を dir、"file" なら worktree と同じ形の file として置く。"" なら置かない。
func makeRepo(t *testing.T, gitEntry string) (root, declPath string) {
	t.Helper()
	root = filepath.Join(testDir(t), "repo")
	for _, d := range []string{"tools/x", "data"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	switch gitEntry {
	case "dir":
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	case "file":
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: ../elsewhere\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "data", "in.txt"), []byte("in"), 0o644); err != nil {
		t.Fatal(err)
	}
	declPath = filepath.Join(root, "tools", "x", "outside.toml")
	if err := os.WriteFile(declPath, []byte("requires_git_worktree = true\nfs_roots = [\"data\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, declPath
}

// emptyPath は PATH を test が作った 空の dir だけにする (git の program が引けない環境)。
func emptyPath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", testDir(t))
}

// real の RunTool は .git (dir でも file でも) を宣言の dir から上へ探して 木の根を基準にする。PATH に git が無くても 同じ。
func TestNoToolName_TC_OUT_NO_TOOL_NAME_RunTool(t *testing.T) {
	for _, gitEntry := range []string{"dir", "file"} {
		for _, noGit := range []bool{false, true} {
			name := ".git が " + gitEntry
			if noGit {
				name += " / PATH に git が無い"
			}
			t.Run(name, func(t *testing.T) {
				_, declPath := makeRepo(t, gitEntry)
				if noGit {
					emptyPath(t)
				}
				called := 0
				err := outside.RunTool(declPath, func(s outside.Set) error {
					called++
					// 宣言の相対 path は 木の根が基準 (宣言の dir の tools/x ではない)。
					if data, err := s.FS.ReadFile("data/in.txt"); err != nil || string(data) != "in" {
						t.Errorf("木の根の data/in.txt = %q, %v", data, err)
					}
					if s.SysInfo == nil || s.IdGen == nil {
						t.Error("real の Set に SysInfo か IdGen が無い")
					}
					return nil
				})
				if err != nil || called != 1 {
					t.Fatalf("RunTool = %v, 呼ばれた回数 %d", err, called)
				}
			})
		}
	}
}

// .git の無い木は git の木が足りないと名乗る。PATH に git が無くても 同じ。
// real の探し方は 上限 (test の dir) で区切って確かめる (test の dir の上には この repo の .git が在るため)。
func TestNoToolName_TC_OUT_NO_TOOL_NAME_無い木(t *testing.T) {
	for _, noGit := range []bool{false, true} {
		root, declPath := makeRepo(t, "")
		if noGit {
			emptyPath(t)
		}
		if got, err := outside.WorktreeRoot(filepath.Dir(declPath), root); err == nil {
			t.Errorf("PATH に git が無い=%v: .git の無い木で 根が引けた: %q", noGit, got)
		}
		// 上限の中に .git が在れば 引ける (陽性対照)。
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got, err := outside.WorktreeRoot(filepath.Dir(declPath), root); err != nil || got != root {
			t.Errorf("PATH に git が無い=%v: WorktreeRoot = %q, %v (want %q)", noGit, got, err, root)
		}
	}

	// RunToolWith: Worktree が nil か 誤りなら 処理を呼ばず git の木が足りないと名乗る (判定不能)。
	decl := outside.Decl{RequiresGitWorktree: true}
	for name, wt := range map[string]func() (string, error){
		"口が無い": nil,
		"誤り":   func() (string, error) { return "", errors.New(".git が無い") },
	} {
		called := 0
		err := outside.RunToolWith(decl, outside.Base{Worktree: wt}, func(outside.Set) error {
			called++
			return nil
		})
		if called != 0 || err == nil || !strings.Contains(err.Error(), "git の木") {
			t.Errorf("%s: 呼ばれた回数 %d, err = %v", name, called, err)
		}
		if code := outside.ExitCode(err); code != outside.ExitUndetermined {
			t.Errorf("%s: exit の番号 = %d (want %d)", name, code, outside.ExitUndetermined)
		}
	}
	// requires_git_worktree が false なら Worktree は問わない。
	fsys, err := outsidemock.NewFS(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := outside.RunToolWith(outside.Decl{}, outside.Base{FS: fsys}, func(outside.Set) error { return nil }); err != nil {
		t.Errorf("git の木を求めない宣言で 誤り: %v", err)
	}
}
