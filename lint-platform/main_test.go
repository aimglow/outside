package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

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
	dir, err := os.MkdirTemp(base, "lint-platform-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// tree は dir の下に 相対 path → 中身 の file を作る。
func tree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func runCmd(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

const goInteg = "//go:build integration\n\npackage x\n"

// TC-OUT-LINT-PLATFORM-TESTS (main の tests を test が作った木で回す)
func TestTests_TC_OUT_LINT_PLATFORM_TESTS(t *testing.T) {
	repo := testDir(t)
	tree(t, repo, map[string]string{
		"libs/platform/db-postgres-go/x_test.go":                  goInteg,
		"tools/dbgate/db_test.go":                                 goInteg,
		"tools/dbgate/unit_test.go":                               "package dbgate\n",
		"apps/web/a.integration.spec.ts":                          "it('x', () => {});\n",
		"libs/platform/http-fetch-next/src/b.integration.spec.ts": "",
		"apps/mobile/test/c_test.dart":                            "@Tags(['integration'])\nlibrary;\n",
		"tools/legacy/d_test.go":                                  "//go:build integration && linux\n\npackage legacy\n",
		"node_modules/pkg/e.integration.spec.ts":                  "",
		"libs/outside/go/g_test.go":                               goInteg,
		"apps/mobile/.dart_tool/h_test.dart":                      "@Tags(['integration'])\n",
		"outside-platform.toml":                                   "capabilities = [\"pay\"]\nadapter_dirs = [\"tools/legacy\"]\n",
	})
	// symlink の dir は辿らない (辿れば 置き場の外に 印つきの test が見える)。
	if err := os.Symlink(filepath.Join(repo, "tools", "dbgate"), filepath.Join(repo, "apps", "linked")); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := runCmd(t, "tests", "--repo", repo)
	if code != 1 {
		t.Fatalf("exit = %d want 1\n%s%s", code, out, errOut)
	}
	var got []string
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		got = append(got, line)
	}
	want := []string{
		"違反 apps/mobile/test/c_test.dart: dart の integration の test は 置き場 (platform_dir と adapter_dirs) の外 (test-classification.md §4)",
		"違反 apps/web/a.integration.spec.ts: ts の integration の test は 置き場 (platform_dir と adapter_dirs) の外 (test-classification.md §4)",
		"違反 tools/dbgate/db_test.go: go の integration の test は 置き場 (platform_dir と adapter_dirs) の外 (test-classification.md §4)",
	}
	if !slices.Equal(got, want) {
		t.Errorf("出力 =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// 宣言を明示しても同じ。宣言の file を外すと tools/legacy も名指す (宣言なしで動く)。
	if code, _, _ := runCmd(t, "tests", "--repo", repo, "--vocab", filepath.Join(repo, "outside-platform.toml")); code != 1 {
		t.Errorf("--vocab 明示の exit = %d", code)
	}
	if err := os.Remove(filepath.Join(repo, "outside-platform.toml")); err != nil {
		t.Fatal(err)
	}
	code, out, _ = runCmd(t, "tests", "--repo", repo)
	if code != 1 || strings.Count(out, "違反 ") != 4 || !strings.Contains(out, "違反 tools/legacy/d_test.go: go ") {
		t.Errorf("宣言なし: exit = %d\n%s", code, out)
	}
}

// 宣言なしで 置き場の外に何も無ければ 0。
func TestTests_Clean(t *testing.T) {
	repo := testDir(t)
	tree(t, repo, map[string]string{
		"libs/platform/db-postgres-go/x_test.go": goInteg,
		"apps/api/unit_test.go":                  "package api\n",
		"apps/web/a.spec.ts":                     "",
	})
	if code, out, errOut := runCmd(t, "tests", "--repo", repo); code != 0 {
		t.Errorf("exit = %d\n%s%s", code, out, errOut)
	}
}

// 宣言の誤り・読めない式・木が無いは 判定不能 (3)。
func TestTests_Unjudgeable(t *testing.T) {
	for _, decl := range []string{
		`adapter_dirs = ["."]`, `adapter_dirs = ["../x"]`, `adapter_dirs = [""]`, `adapter_dirs = ["/abs"]`,
		`unknown = 1`, `adapter_dirs = "tools"`,
	} {
		repo := testDir(t)
		tree(t, repo, map[string]string{"outside-platform.toml": decl + "\n"})
		if code, out, errOut := runCmd(t, "tests", "--repo", repo); code != 3 {
			t.Errorf("宣言 %s: exit = %d want 3\n%s%s", decl, code, out, errOut)
		}
	}

	repo := testDir(t)
	tree(t, repo, map[string]string{"apps/x/a_test.go": "//go:build (\n\npackage x\n"})
	code, out, _ := runCmd(t, "tests", "--repo", repo)
	if code != 3 || !strings.Contains(out, "判定不能 apps/x/a_test.go: ") {
		t.Errorf("読めない式: exit = %d\n%s", code, out)
	}

	if code, _, _ := runCmd(t, "tests", "--repo", filepath.Join(repo, "none")); code != 3 {
		t.Errorf("無い木: exit = %d", code)
	}
	if code, _, _ := runCmd(t, "tests"); code != 3 {
		t.Errorf("--repo なし: exit = %d", code)
	}
}

// 読めない file は 判定不能 (3) で path を名乗る。
func TestTests_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root は 権限を外しても読める")
	}
	repo := testDir(t)
	tree(t, repo, map[string]string{"apps/x/a_test.go": goInteg})
	p := filepath.Join(repo, "apps", "x", "a_test.go")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	code, out, _ := runCmd(t, "tests", "--repo", repo)
	if code != 3 || !strings.Contains(out, "判定不能 apps/x/a_test.go: 読めない") {
		t.Errorf("exit = %d\n%s", code, out)
	}
}

// check は adapter_dirs を持つ宣言でも 3 にならない (宣言の file は check と tests が共に読む)。
func TestCheck_AcceptsAdapterDirs(t *testing.T) {
	dir := testDir(t)
	tree(t, dir, map[string]string{
		"libs/platform/pay-stripe-go/interface.go":    "package x\n",
		"libs/platform/pay-stripe-go/adapter_real.go": "package x\n",
		"libs/platform/pay-stripe-go/adapter_mock.go": "package x\n",
		"outside-platform.toml":                       "capabilities = [\"pay\"]\nadapter_dirs = [\"tools/legacy\"]\n",
	})
	code, out, errOut := runCmd(t, "check", "--root", filepath.Join(dir, "libs", "platform"), "--vocab", filepath.Join(dir, "outside-platform.toml"))
	if code != 0 {
		t.Errorf("exit = %d\n%s%s", code, out, errOut)
	}
	tree(t, dir, map[string]string{"outside-platform.toml": "capabilities = [\"pay\"]\nother = 1\n"})
	if code, _, _ := runCmd(t, "check", "--root", filepath.Join(dir, "libs", "platform"), "--vocab", filepath.Join(dir, "outside-platform.toml")); code != 3 {
		t.Errorf("知らない key: exit = %d want 3", code)
	}
}

// TC-OUT-LINT-PLATFORM-TESTS (本物の口の分を main の tests で回す)
func TestTests_RealUse_TC_OUT_LINT_PLATFORM_TESTS(t *testing.T) {
	repo := testDir(t)
	tree(t, repo, map[string]string{
		"go.mod": "module example.com/proj\n",
		"libs/platform/db-postgres-go/adapter_real.go": "package dbpostgres\n\ntype Real struct{}\n",
		"libs/platform/db-postgres-go/adapter_mock.go": "package dbpostgres\n\ntype Mock struct{}\n",
		"libs/platform/db-postgres-go/real_test.go":    "package dbpostgres\n\nimport \"github.com/aimglow/outside/go\"\n\nvar p = outside.NewProc(nil)\n",
		"libs/platform/http-fetch-next/src/real.ts":    "export class Real {}\n",
		"tools/x/a_test.go":                            "package x\n\nimport \"github.com/aimglow/outside/go\"\n\nvar p = outside.NewProc(nil)\n",
		"apps/web/b.spec.ts":                           "import { createRealSet as rs } from '@aimglow/outside-next';\n",
		"apps/mobile/test/c2_test.dart":                "import 'package:outside/outside.dart';\n\nfinal h = RealHttp();\n",
		"tools/y/d_test.go":                            "package y\n\nimport \"example.com/proj/libs/platform/db-postgres-go\"\n\nvar r = dbpostgres.Real{}\n",
		"apps/web/e.spec.ts":                           "import { Real } from './real';\n",
		"tools/x/m_test.go":                            "package x\n\nimport \"github.com/aimglow/outside/go/outsidemock\"\n\nvar p = outsidemock.NewProc(nil)\n",
		"tools/y/m_test.go":                            "package y\n\nimport \"example.com/proj/libs/platform/db-postgres-go\"\n\nvar m = dbpostgres.Mock{}\n",
		"apps/web/m.spec.ts":                           "import { createMockSet } from '@aimglow/outside-next/mock';\n",
		"apps/mobile/test/m_test.dart":                 "import 'package:outside/mock.dart';\n\nfinal h = MockHttp();\n",
		"libs/outside/go/x_test.go":                    "package outside\n\nvar p = NewProc(nil)\n",
	})
	code, out, errOut := runCmd(t, "tests", "--repo", repo)
	if code != 1 {
		t.Fatalf("exit = %d want 1\n%s%s", code, out, errOut)
	}
	want := []string{
		"違反 apps/mobile/test/c2_test.dart:3: dart の test が 本物の口 RealHttp を使う (置き場の外。test-classification.md §4)",
		"違反 apps/web/b.spec.ts:1: ts の test が 本物の口 createRealSet を使う (置き場の外。test-classification.md §4)",
		"違反 apps/web/e.spec.ts:1: ts の test が 本物の口 ./real を使う (置き場の外。test-classification.md §4)",
		"違反 tools/x/a_test.go:5: go の test が 本物の口 outside.NewProc を使う (置き場の外。test-classification.md §4)",
		"違反 tools/y/d_test.go:5: go の test が 本物の口 dbpostgres.Real を使う (置き場の外。test-classification.md §4)",
	}
	if got := strings.Split(strings.TrimSpace(out), "\n"); !slices.Equal(got, want) {
		t.Errorf("出力 =\n%s\nwant\n%s", out, strings.Join(want, "\n"))
	}
}

// TC-OUT-LINT-PLATFORM-PLATFORM-DIR: 宣言 platform_dir が adapters の置き場を決める。
// 書かない = 既定 libs/platform / "" = 置き場を持たない (adapter_dirs だけ) / path = その dir。
func TestPlatformDir_TC_OUT_LINT_PLATFORM_PLATFORM_DIR(t *testing.T) {
	integ := "//go:build integration\n\npackage be\n"
	files := map[string]string{
		"libs/platform/x/be/real_test.go": integ,
		"adapters/y/real_test.go":         integ,
		"libs/adapters/z/real_test.go":    integ,
	}
	for _, tc := range []struct {
		label    string
		toml     string
		wantCode int
		wantViol []string
	}{
		{"書かない = 既定", "adapter_dirs = [\"adapters\"]\n", 1, []string{"libs/adapters/z/real_test.go"}},
		{"空 = 置き場を持たない", "platform_dir = \"\"\nadapter_dirs = [\"adapters\"]\n", 1, []string{"libs/adapters/z/real_test.go", "libs/platform/x/be/real_test.go"}},
		{"別の dir", "platform_dir = \"libs/adapters\"\nadapter_dirs = [\"adapters\"]\n", 1, []string{"libs/platform/x/be/real_test.go"}},
	} {
		repo := testDir(t)
		fs := map[string]string{"outside-platform.toml": tc.toml}
		for k, v := range files {
			fs[k] = v
		}
		tree(t, repo, fs)
		code, out, errOut := runCmd(t, "tests", "--repo", repo)
		if code != tc.wantCode {
			t.Errorf("%s: exit = %d (want %d)\n%s%s", tc.label, code, tc.wantCode, out, errOut)
		}
		for _, p := range []string{"libs/platform/x/be/real_test.go", "adapters/y/real_test.go", "libs/adapters/z/real_test.go"} {
			want := slices.Contains(tc.wantViol, p)
			if got := strings.Contains(out+errOut, "違反 "+p+":"); got != want {
				t.Errorf("%s: %s の違反 = %v (want %v)\n%s%s", tc.label, p, got, want, out, errOut)
			}
		}
	}
	for _, bad := range []string{`platform_dir = "."`, `platform_dir = "../x"`, `platform_dir = "/abs"`, `platform_dir = 1`} {
		repo := testDir(t)
		tree(t, repo, map[string]string{"outside-platform.toml": bad + "\n"})
		if code, _, _ := runCmd(t, "tests", "--repo", repo); code != 3 {
			t.Errorf("宣言 %s: exit = %d (want 3)", bad, code)
		}
	}
}

// check: platform_dir = "" は 命名の検査をしない (名乗って 0)。宣言と違う --root は 判定不能。scaffold は 置き場が無ければ 3。
func TestCheck_PlatformDir(t *testing.T) {
	repo := testDir(t)
	tree(t, repo, map[string]string{
		"libs/platform/anything-goes/x.txt": "x",
		"outside-platform.toml":             "platform_dir = \"\"\n",
	})
	vocab := filepath.Join(repo, "outside-platform.toml")
	code, out, _ := runCmd(t, "check", "--vocab", vocab)
	if code != 0 || !strings.Contains(out, "命名の検査はしない") {
		t.Errorf("platform_dir = \"\" の check: exit = %d out = %q (want 0 と 名乗り)", code, out)
	}
	if code, _, errOut := runCmd(t, "check", "--root", "libs/platform", "--vocab", vocab); code != 3 || !strings.Contains(errOut, "違う") {
		t.Errorf("宣言と違う --root: exit = %d err = %q (want 3)", code, errOut)
	}
	if code, _, _ := runCmd(t, "scaffold", "--vocab", vocab, "db", "postgres", "go"); code != 3 {
		t.Errorf("置き場を持たない scaffold (--root なし): exit = %d (want 3)", code)
	}
	// 宣言で書いた置き場と同じ --root は 食い違いにならない (置き場が無いので 読めない = 3 だが 食い違いの名乗りではない)。
	tree(t, repo, map[string]string{"outside-platform.toml": "platform_dir = \"libs/adapters\"\n"})
	if _, _, errOut := runCmd(t, "check", "--root", "libs/adapters", "--vocab", vocab); strings.Contains(errOut, "違う") {
		t.Errorf("宣言と同じ --root を 食い違いと読んだ: %q", errOut)
	}
}
