package placement

import (
	"reflect"
	"testing"
)

func decl(t *testing.T, dirs ...string) Decl {
	t.Helper()
	d, err := NewDecl(dirs)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

const goInteg = "//go:build integration\n\npackage x\n"

// TC-OUT-LINT-PLATFORM-TESTS (spec の input の 印の分)
func TestCheck_TC_OUT_LINT_PLATFORM_TESTS(t *testing.T) {
	files := []File{
		{"libs/platform/db-postgres-go/x_test.go", []byte(goInteg)},
		{"tools/dbgate/db_test.go", []byte(goInteg)},
		{"tools/dbgate/unit_test.go", []byte("package dbgate\n")},
		{"apps/web/a.integration.spec.ts", []byte("it('x', () => {});\n")},
		{"libs/platform/http-fetch-next/src/b.integration.spec.ts", []byte("")},
		{"apps/mobile/test/c_test.dart", []byte("@Tags(['integration'])\nlibrary;\n")},
		{"tools/legacy/d_test.go", []byte("//go:build integration && linux\n\npackage legacy\n")},
		{"node_modules/pkg/e.integration.spec.ts", []byte("")},
		{"apps/web/node_modules/pkg/f_test.go", []byte(goInteg)},
		{"libs/outside/go/g_test.go", []byte(goInteg)},
		{"apps/mobile/.dart_tool/h_test.dart", []byte("@Tags(['integration'])\n")},
		{".git/i_test.go", []byte(goInteg)},
	}
	r := Check(files, decl(t, "tools/legacy"))
	want := []Finding{
		{Path: "apps/mobile/test/c_test.dart", Runtime: "dart"},
		{Path: "apps/web/a.integration.spec.ts", Runtime: "ts"},
		{Path: "tools/dbgate/db_test.go", Runtime: "go"},
	}
	if !reflect.DeepEqual(r.Violations, want) || len(r.Unknowns) != 0 {
		t.Errorf("違反 = %v 判定不能 = %v\nwant %v", r.Violations, r.Unknowns, want)
	}
	if got := r.Violations[2].String(); got != "tools/dbgate/db_test.go: go の integration の test は 置き場 (libs/platform/** と adapter_dirs) の外 (test-classification.md §4)" {
		t.Errorf("名指しの文 = %q", got)
	}
}

// 置き場は path の区切りで比べる (prefix の文字列一致で 隣の dir を含めない)。
func TestAllowed_Boundary(t *testing.T) {
	d := decl(t, "tools/dbgate", "./apps/legacy/")
	cases := map[string]bool{
		"tools/dbgate/x_test.go":          true,
		"tools/dbgate/sub/x_test.go":      true,
		"tools/dbgate2/x_test.go":         false,
		"tools/dbgate.old/x_test.go":      false,
		"tools/x_test.go":                 false,
		"apps/legacy/a_test.go":           true,
		"apps/legacy2/a_test.go":          false,
		"libs/platform/db-x-go/a_test.go": true,
		"libs/platformx/a_test.go":        false,
		"libs/a_test.go":                  false,
	}
	for p, want := range cases {
		if got := Allowed(p, d); got != want {
			t.Errorf("Allowed(%q) = %v want %v", p, got, want)
		}
	}
	if got := d.AdapterDirs(); !reflect.DeepEqual(got, []string{"apps/legacy", "tools/dbgate"}) {
		t.Errorf("揃えた宣言 = %v", got)
	}
}

func TestSkipDir(t *testing.T) {
	cases := map[string]bool{
		".git": true, "node_modules": true, "apps/web/node_modules": true, "apps/m/.dart_tool": true,
		"libs/outside": true, "libs/outside/go": true,
		"libs/outsidex": false, "apps/outside": false, "libs/platform": false, "node_modules_x": false,
	}
	for p, want := range cases {
		if got := SkipDir(p); got != want {
			t.Errorf("SkipDir(%q) = %v want %v", p, got, want)
		}
	}
}

// 宣言の誤り (空・.・絶対 path・根の外) は 判定不能。
func TestNewDecl_Invalid(t *testing.T) {
	for _, d := range []string{"", " ", ".", "./", "/abs/dir", "..", "../x", "a/../..", "tools/../."} {
		if _, err := NewDecl([]string{d}); err == nil {
			t.Errorf("宣言 %q が通った", d)
		}
	}
	if _, err := NewDecl(nil); err != nil {
		t.Errorf("宣言なし = %v", err)
	}
}

// Go の印 = //go:build の式で tag integration が 肯定で効くか。
func TestGoMarked(t *testing.T) {
	cases := []struct {
		name, src string
		want      bool
	}{
		{"integration", "//go:build integration\n\npackage x\n", true},
		{"integration && linux", "//go:build integration && linux\n\npackage x\n", true},
		{"unit || integration", "//go:build unit || integration\n\npackage x\n", true},
		{"!(!integration)", "//go:build !(!integration)\n\npackage x\n", true},
		{"!integration は印でない", "//go:build !integration\n\npackage x\n", false},
		{"integration を含まない式", "//go:build linux\n\npackage x\n", false},
		{"満たせない式", "//go:build integration && !integration\n\npackage x\n", false},
		{"integration || !integration は効かない", "//go:build integration || !integration\n\npackage x\n", false},
		{"式なし", "package x\n", false},
		{"注釈の後の式", "// Copyright\n\n//go:build integration\n\npackage x\n", true},
		{"package の後の行は式でない", "package x\n\n//go:build integration\n", false},
		{"似た名前の tag", "//go:build integration_slow\n\npackage x\n", false},
		{"// +build だけ", "// +build integration\n\npackage x\n", true},
		{"// +build の行の積", "// +build integration\n// +build !integration\n\npackage x\n", false},
		{"//go:build が // +build に勝つ", "//go:build linux\n// +build integration\n\npackage x\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Marked("go", []byte(c.src))
			if err != nil || got != c.want {
				t.Errorf("Marked = %v, %v want %v", got, err, c.want)
			}
		})
	}
	for _, src := range []string{"//go:build integration &&\n\npackage x\n", "//go:build integration\n//go:build linux\n\npackage x\n"} {
		if _, err := Marked("go", []byte(src)); err == nil {
			t.Errorf("読めない式が 誤りにならない: %q", src)
		}
	}
	r := Check([]File{{"apps/x/a_test.go", []byte("//go:build (\n\npackage x\n")}}, Decl{})
	if len(r.Unknowns) != 1 || r.Unknowns[0].Path != "apps/x/a_test.go" || len(r.Violations) != 0 {
		t.Errorf("読めない式 = %v / %v (want 判定不能 1 件)", r.Unknowns, r.Violations)
	}
}

// Dart の印 = 名前が *_test.dart で @Tags か tags: に 'integration' か "integration"。
func TestDartMarked(t *testing.T) {
	cases := []struct {
		name, src string
		want      bool
	}{
		{"@Tags 一重引用", "@Tags(['integration'])\nlibrary;\n", true},
		{"@Tags 二重引用", "@Tags([\"integration\"])\nlibrary;\n", true},
		{"@Tags 複数行", "@Tags([\n  'slow',\n  'integration',\n])\nlibrary;\n", true},
		{"tags: 文字列", "test('x', () {}, tags: 'integration');\n", true},
		{"tags: list", "group('x', () {}, tags: [\"integration\", 'slow']);\n", true},
		{"tags: const list", "test('x', () {}, tags: const ['integration']);\n", true},
		{"印なし", "test('x', () {});\n", false},
		{"別の tag", "@Tags(['integration_slow'])\nlibrary;\n", false},
		{"本文の語", "// integration の試験\ntest('integration', () {});\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, _ := Marked("dart", []byte(c.src)); got != c.want {
				t.Errorf("Marked = %v want %v", got, c.want)
			}
		})
	}
}

// TS の印 = 名前。dart と go は名前で候補になり 中身で決まる。
func TestCandidate(t *testing.T) {
	cases := map[string]string{
		"a/x.integration.spec.ts":   "ts",
		"a/x.integration.test.tsx":  "ts",
		"a/x.integration.spec.js":   "ts",
		"a/x.integration.test.mjs":  "ts",
		"a/x.spec.ts":               "",
		"a/x.integration.ts":        "",
		"a/x.integration.spec.d.ts": "",
		"a/x_test.go":               "go",
		"a/x.go":                    "",
		"a/x_test.dart":             "dart",
		"a/x.dart":                  "",
	}
	for p, want := range cases {
		got, ok := Candidate(p)
		if got != want || ok != (want != "") {
			t.Errorf("Candidate(%q) = %q, %v want %q", p, got, ok, want)
		}
	}
}
