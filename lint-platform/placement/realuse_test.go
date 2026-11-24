package placement

import (
	"reflect"
	"slices"
	"testing"

	"github.com/aimglow/outside/lint-platform/naming"
)

// realTree は TC-OUT-LINT-PLATFORM-TESTS の 本物の口の分の木 (adapters の本物と go.mod・pubspec.yaml を含む)。
func realTree() []File {
	return []File{
		{"go.mod", []byte("module example.com/proj\n\ngo 1.26\n")},
		{"libs/platform/db-postgres-go/adapter_real.go", []byte("package dbpostgres\n\ntype Real struct{}\n\nfunc NewReal() *Real { return &Real{} }\n\nfunc (r *Real) Query() {}\n\nvar DefaultTimeout = 1\n\nfunc helper() {}\n")},
		{"libs/platform/http-fetch-next/src/real.ts", []byte("import type { Port } from './interface';\n\nexport class Real implements Port {}\nexport function openReal(): Real { return new Real(); }\n")},
		{"libs/platform/kv-prefs-flutter/pubspec.yaml", []byte("name: kv_prefs_flutter\n")},
		{"libs/platform/kv-prefs-flutter/lib/adapter_real.dart", []byte("import 'interface.dart';\n\nfinal class RealPrefs implements Port {}\n\nRealPrefs openPrefs() {\n  return RealPrefs();\n}\n")},

		// 本物の口を使う (名指す)
		{"tools/x/a_test.go", []byte("package x\n\nimport (\n\t\"testing\"\n\n\t\"github.com/aimglow/outside/go\"\n)\n\nfunc TestA(t *testing.T) {\n\t_ = outside.NewProc(nil)\n}\n")},
		{"tools/x/b_test.go", []byte("package x\n\nimport o \"github.com/aimglow/outside/go\"\n\nvar c = o.NewClock()\n")},
		{"tools/x/c_test.go", []byte("package x\n\nimport . \"github.com/aimglow/outside/go\"\n\nvar e = NewEnv()\n")},
		{"apps/web/b.spec.ts", []byte("import { describe } from 'vitest';\nimport {\n  createMockClock,\n  createRealSet as rs,\n} from '@aimglow/outside-next';\n\nrs();\n")},
		{"apps/web/ns.spec.ts", []byte("import * as o from '../../libs/outside/angular/src';\n\no.createMockClock(0);\no.createRealClock();\n")},
		{"apps/mobile/test/c2_test.dart", []byte("import 'package:outside/outside.dart';\n\nvoid main() {\n  final h = RealHttp();\n  // RealClock() は使わない\n  final s = OutsideSet.real();\n}\n")},
		{"tools/y/d_test.go", []byte("package y\n\nimport db \"example.com/proj/libs/platform/db-postgres-go\"\n\nvar r = db.Real{}\nvar m = db.Mock{}\nvar n = db.NewReal()\n")},
		{"apps/web/e.spec.ts", []byte("import { Real } from './real';\n")},
		{"apps/web/f.spec.ts", []byte("import { Mock, Real } from '@proj/http-fetch-next';\nimport type { Real as R } from '@proj/http-fetch-next/src/real';\n")},
		{"apps/mobile/test/k_test.dart", []byte("import 'package:kv_prefs_flutter/kv_prefs_flutter.dart';\nimport 'package:kv_prefs_flutter/adapter_real.dart';\n\nvoid main() {\n  test('RealPrefs works', () {\n    final p = RealPrefs();\n    final q = MockPrefs();\n  });\n}\n")},

		// mock だけ (名指さない)
		{"tools/x/m_test.go", []byte("package x\n\nimport (\n\t\"github.com/aimglow/outside/go\"\n\t\"github.com/aimglow/outside/go/outsidemock\"\n)\n\nvar p = outsidemock.NewProc(nil)\nvar c outside.Clock\n")},
		{"tools/y/m_test.go", []byte("package y\n\nimport \"example.com/proj/libs/platform/db-postgres-go\"\n\nvar m = dbpostgres.Mock{}\n\ntype s struct{ NewReal int }\n\nvar v = s{}.NewReal\n")},
		{"apps/web/m.spec.ts", []byte("import { createMockSet } from '@aimglow/outside-next/mock';\nimport { Mock } from '../../libs/platform/http-fetch-next/src/mock';\nimport { createRealSet } from './helpers';\n")},
		{"apps/mobile/test/m_test.dart", []byte("import 'package:outside/mock.dart';\nimport 'package:kv_prefs_flutter/adapter_mock.dart';\n\nvoid main() {\n  final h = MockHttp();\n  final p = MockPrefs();\n}\n")},
		{"apps/mobile/test/n_test.dart", []byte("void main() {\n  final h = RealHttp();\n}\n")}, // outside を import しない

		// 置き場の中で本物を使う・写しの下 (名指さない)
		{"libs/platform/db-postgres-go/real_test.go", []byte("package dbpostgres\n\nimport \"github.com/aimglow/outside/go\"\n\nvar p = outside.NewProc(nil)\n")},
		{"tools/legacy/l_test.go", []byte("package legacy\n\nimport \"github.com/aimglow/outside/go\"\n\nvar p = outside.NewProc(nil)\n")},
		{"libs/outside/go/x_test.go", []byte("package outside\n\nvar p = NewProc(nil)\n")},
		{"libs/platform/http-fetch-next/src/real.spec.ts", []byte("import { Real } from './real';\n")},
	}
}

// TC-OUT-LINT-PLATFORM-TESTS (spec の input の 本物の口の分)
func TestCheck_RealUse_TC_OUT_LINT_PLATFORM_TESTS(t *testing.T) {
	r := Check(realTree(), decl(t, "tools/legacy"))
	want := []Finding{
		{"apps/mobile/test/c2_test.dart", 4, "dart", "RealHttp"},
		{"apps/mobile/test/c2_test.dart", 6, "dart", "OutsideSet.real"},
		{"apps/mobile/test/k_test.dart", 2, "dart", "package:kv_prefs_flutter/adapter_real.dart"},
		{"apps/mobile/test/k_test.dart", 6, "dart", "kv-prefs-flutter.RealPrefs"},
		{"apps/web/b.spec.ts", 4, "ts", "createRealSet"},
		{"apps/web/e.spec.ts", 1, "ts", "./real"},
		{"apps/web/f.spec.ts", 1, "ts", "http-fetch-next.Real"},
		{"apps/web/ns.spec.ts", 4, "ts", "createRealClock"},
		{"tools/x/a_test.go", 10, "go", "outside.NewProc"},
		{"tools/x/b_test.go", 5, "go", "outside.NewClock"},
		{"tools/x/c_test.go", 5, "go", "outside.NewEnv"},
		{"tools/y/d_test.go", 5, "go", "dbpostgres.Real"},
		{"tools/y/d_test.go", 7, "go", "dbpostgres.NewReal"},
	}
	if !reflect.DeepEqual(r.Violations, want) || len(r.Unknowns) != 0 {
		t.Errorf("違反 =\n%v\n判定不能 = %v\nwant\n%v", r.Violations, r.Unknowns, want)
	}
	if got := want[8].String(); got != "tools/x/a_test.go:10: go の test が 本物の口 outside.NewProc を使う (置き場の外。test-classification.md §4)" {
		t.Errorf("名指しの文 = %q", got)
	}
}

// 本物の file の名前は 雛形の決まり (naming.RequiredFiles) と同じ。outside の表は 本物だけ。
func TestRulesTable(t *testing.T) {
	if naming.RequiredFiles["next"][1] != naming.RequiredFiles["angular"][1] {
		t.Errorf("next と angular の 本物の file が違う: %v", naming.RequiredFiles)
	}
	want := map[string]string{"go": "adapter_real.go", "ts": "real.ts", "dart": "adapter_real.dart"}
	if !reflect.DeepEqual(AdapterRealFiles, want) {
		t.Errorf("AdapterRealFiles = %v", AdapterRealFiles)
	}
	for _, n := range OutsideGoReal {
		if !(n == "RunTool" || len(n) > 3 && n[:3] == "New") {
			t.Errorf("outside の Go の本物の表に %s", n)
		}
	}
}

// adapters の本物が go.mod の下に無ければ import path を決められない = 判定不能。
func TestCheck_GoAdapterWithoutModule(t *testing.T) {
	r := Check([]File{{"libs/platform/db-x-go/adapter_real.go", []byte("package dbx\n")}}, Decl{})
	if len(r.Unknowns) != 1 || r.Unknowns[0].Path != "libs/platform/db-x-go/adapter_real.go" {
		t.Errorf("判定不能 = %v", r.Unknowns)
	}
	r = Check([]File{{"tools/a_test.go", []byte("package a\nfunc (")}}, Decl{})
	if len(r.Unknowns) != 1 || r.Unknowns[0].Path != "tools/a_test.go" {
		t.Errorf("読めない test の 判定不能 = %v", r.Unknowns)
	}
}

// lib 毎の go.mod なら その module path で import する。
func TestCheck_GoAdapterOwnModule(t *testing.T) {
	r := Check([]File{
		{"libs/platform/db-x-go/go.mod", []byte("module dbx\n")},
		{"libs/platform/db-x-go/adapter_real.go", []byte("package dbx\n\nfunc NewReal() {}\n")},
		{"apps/api/a_test.go", []byte("package api\n\nimport \"dbx\"\n\nvar _ = dbx.NewReal\n")},
	}, Decl{})
	if len(r.Violations) != 1 || r.Violations[0].Name != "dbx.NewReal" {
		t.Errorf("違反 = %v 判定不能 = %v", r.Violations, r.Unknowns)
	}
}

func TestModuleAndPubspec(t *testing.T) {
	if got := ModulePath([]byte("// c\nmodule \"a.b/c\" // x\n")); got != "a.b/c" {
		t.Errorf("ModulePath = %q", got)
	}
	if got := PubspecName([]byte("description: x\nname: kv_x\nversion: 1\n")); got != "kv_x" {
		t.Errorf("PubspecName = %q", got)
	}
}

func TestDeclaredNames(t *testing.T) {
	ts := tsExported([]byte("export class Real {}\nexport const make = () => 1;\nexport interface Port {}\nexport type T = 1;\nexport { a as b, type C };\nexport default function d() {}\n"))
	slices.Sort(ts)
	if want := []string{"Real", "b", "d", "make"}; !slices.Equal(ts, want) {
		t.Errorf("tsExported = %v want %v", ts, want)
	}
	dart := dartDeclared([]byte("import 'x.dart';\n\nfinal class RealA implements P {\n  void run() {}\n}\nabstract class B {}\nFuture<int> openA(String s) async {\n  if (s.isEmpty) {}\n}\nvoid _hidden() {}\n"))
	if want := []string{"RealA", "B", "openA"}; !slices.Equal(dart, want) {
		t.Errorf("dartDeclared = %v want %v", dart, want)
	}
}

func TestTestFile(t *testing.T) {
	cases := map[string]string{
		"a/x.spec.ts": "ts", "a/x.test.tsx": "ts", "a/x.integration.spec.mjs": "ts", "a/x.ts": "",
		"a/x_test.go": "go", "a/x_test.dart": "dart", "a/x.dart": "",
	}
	for p, want := range cases {
		got, ok := TestFile(p)
		if got != want || ok != (want != "") {
			t.Errorf("TestFile(%q) = %q, %v", p, got, ok)
		}
	}
}
