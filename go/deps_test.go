package outside_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TC-OUT-RUNTIME-DEPS (go 分): go.mod の require は BurntSushi/toml だけ。source の import は 標準と この module と toml だけ。

const module = "github.com/aimglow/outside/go"

// requires は go.mod の require の module の path を返す (1 行の形と 括弧の塊の形の両方)。
func requires(gomod string) []string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(gomod, "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		switch {
		case inBlock && line == ")":
			inBlock = false
		case inBlock && line != "":
			out = append(out, strings.Fields(line)[0])
		case line == "require (":
			inBlock = true
		case strings.HasPrefix(line, "require "):
			out = append(out, strings.Fields(line)[1])
		}
	}
	return out
}

func TestRuntimeDeps_TC_OUT_RUNTIME_DEPS_go(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if got := requires(string(data)); !reflect.DeepEqual(got, []string{"github.com/BurntSushi/toml"}) {
		t.Errorf("go.mod の require = %v (want [github.com/BurntSushi/toml])", got)
	}
	// 陽性対照: 読み方が 括弧の塊の形の 余分な依存を捕る。
	extra := "module x\n\nrequire (\n\tgithub.com/BurntSushi/toml v1.6.0\n\tgolang.org/x/sys v0.1.0 // indirect\n)\n"
	if got := requires(extra); len(got) != 2 {
		t.Errorf("陽性対照の require = %v (want 2 件)", got)
	}

	// source (test を除く) の import。
	fset := token.NewFileSet()
	err = filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			first, _, _ := strings.Cut(p, "/")
			std := !strings.Contains(first, ".")
			if !std && p != "github.com/BurntSushi/toml" && p != module && !strings.HasPrefix(p, module+"/") {
				t.Errorf("%s が 標準の外の %s を import する", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
