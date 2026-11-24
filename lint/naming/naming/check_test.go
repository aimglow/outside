package naming

import (
	"reflect"
	"testing"
)

// TC-OUT-LINT-NAMING: adapters 層の 名前と構成の規則を 値の表で確かめる。disk を読まない。
func TestCheck_TC_OUT_LINT_NAMING(t *testing.T) {
	goLib := []string{"interface.go", "adapter_real.go", "adapter_mock.go"}
	cases := []struct {
		name string
		in   []Entry
		want []Finding
	}{
		{"規則どおり (Go・直下)", []Entry{{Name: "db-postgres-go", Dir: true, Files: goLib}}, nil},
		{"規則どおり (TS・src の下)", []Entry{{Name: "authn-cookie-next", Dir: true,
			Files: []string{"src/interface.ts", "src/real.ts", "src/mock.ts"}, Subdirs: []string{"src"}}}, nil},
		{"規則どおり (Dart・lib の下 + test)", []Entry{{Name: "kv-shared-preferences-flutter", Dir: true,
			Files:   []string{"lib/interface.dart", "lib/adapter_real.dart", "lib/adapter_mock.dart", "test/x_test.dart"},
			Subdirs: []string{"lib", "test"}}}, nil},
		{"機能の語が表に無い", []Entry{{Name: "database-postgres-go", Dir: true, Files: goLib}},
			[]Finding{{"database-postgres-go", "名前が <機能>-<依存先>-<runtime> の形でない (機能 = authn|db|vcs|container|migrate|schema|config|log|http|kv|ipc|remote、runtime = go|next|angular|flutter)"}}},
		{"runtime が無い", []Entry{{Name: "config-go", Dir: true, Files: goLib}},
			[]Finding{{"config-go", "名前が <機能>-<依存先>-<runtime> の形でない (機能 = authn|db|vcs|container|migrate|schema|config|log|http|kv|ipc|remote、runtime = go|next|angular|flutter)"}}},
		{"何の依存先か分からない名前 (pgtest)", []Entry{{Name: "pgtest", Dir: true}},
			[]Finding{{"pgtest", "名前が <機能>-<依存先>-<runtime> の形でない (機能 = authn|db|vcs|container|migrate|schema|config|log|http|kv|ipc|remote、runtime = go|next|angular|flutter)"}}},
		{"mock が無い", []Entry{{Name: "vcs-git-go", Dir: true, Files: []string{"interface.go", "adapter_real.go"}}},
			[]Finding{{"vcs-git-go", "adapter_mock.go が無い (runtime go の 3 file = interface.go・adapter_real.go・adapter_mock.go)"}}},
		{"許さない dir", []Entry{{Name: "vcs-git-go", Dir: true, Files: goLib, Subdirs: []string{"util"}}},
			[]Finding{{"vcs-git-go", "dir util は置かない (置いてよい dir = src・lib・test・testdata)"}}},
		{"許さない dir の中の 3 file は数えない", []Entry{{Name: "vcs-git-go", Dir: true,
			Files: []string{"util/interface.go", "adapter_real.go", "adapter_mock.go"}, Subdirs: []string{"util"}}},
			[]Finding{{"vcs-git-go", "interface.go が無い (runtime go の 3 file = interface.go・adapter_real.go・adapter_mock.go)"},
				{"vcs-git-go", "dir util は置かない (置いてよい dir = src・lib・test・testdata)"}}},
		{"直下の file", []Entry{{Name: "README.md"}}, []Finding{{"README.md", "lib の dir 以外の file は置かない"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Check(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Check =\n%v\nwant\n%v", got, c.want)
			}
		})
	}
}

// 陽性対照: 規則の表に在る語と runtime は 全部 名前として通る (表と正規表現の食い違いを捕る)。
func TestRulesTableAndPatternAgree(t *testing.T) {
	for _, c := range Capabilities {
		for _, r := range Runtimes {
			n := c + "-x-" + r
			if !namePattern.MatchString(n) {
				t.Errorf("表に在る %s が 正規表現で通らない", n)
			}
		}
	}
	for _, r := range Runtimes {
		if len(RequiredFiles[r]) != 3 {
			t.Errorf("runtime %s の 3 file が %d 個", r, len(RequiredFiles[r]))
		}
	}
}
