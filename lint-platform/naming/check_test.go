package naming

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func vocab(t *testing.T, declared ...string) Vocab {
	t.Helper()
	v, err := NewVocab(declared)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TC-OUT-LINT-PLATFORM
func TestCheck_TC_OUT_LINT_PLATFORM(t *testing.T) {
	goLib := []string{"adapter_mock.go", "adapter_real.go", "interface.go"}
	bad := func(n string) Finding {
		return Finding{n, "名前が <機能>-<依存先>-<runtime> の形でない (機能 = authn|config|container|db|http|ipc|kv|log|mail|migrate|pdf|push|remote|schema|secret|storage|trace|vcs、runtime = go|next|angular|flutter。project 固有の機能は outside-platform.toml に宣言する)"}
	}
	cases := []struct {
		name     string
		in       []Entry
		want     []Finding
		wantWarn int
	}{
		{"Go・直下", []Entry{{Name: "db-postgres-go", Dir: true, Found: goLib}}, nil, 0},
		{"TS・src の下", []Entry{{Name: "authn-cookie-next", Dir: true, Found: []string{"src/interface.ts", "src/mock.ts", "src/real.ts"}}}, nil, 0},
		{"Dart・lib の下", []Entry{{Name: "kv-shared-preferences-flutter", Dir: true, Found: []string{"lib/adapter_mock.dart", "lib/adapter_real.dart", "lib/interface.dart"}}}, nil, 0},
		{"表に無い機能の語", []Entry{{Name: "database-postgres-go", Dir: true, Found: goLib}}, []Finding{bad("database-postgres-go")}, 0},
		{"runtime が無い", []Entry{{Name: "config-go", Dir: true, Found: goLib}}, []Finding{bad("config-go")}, 0},
		{"何の依存先か分からない名前", []Entry{{Name: "pgtest", Dir: true}}, []Finding{bad("pgtest")}, 0},
		{"機能に中身を表さない語", []Entry{{Name: "common-postgres-go", Dir: true, Found: goLib}}, []Finding{bad("common-postgres-go")}, 0},
		{"mock が無い", []Entry{{Name: "vcs-git-go", Dir: true, Found: []string{"adapter_real.go", "interface.go"}}},
			[]Finding{{"vcs-git-go", "adapter_mock.go が無い (runtime go の 3 file = interface.go・adapter_real.go・adapter_mock.go を 直下か src/ か lib/ に置く)"}}, 0},
		{"直下の file", []Entry{{Name: "README.md"}}, []Finding{{"README.md", "lib の dir 以外の物は 直下に置かない"}}, 0},
		{"sub dir と設定の file は見ない (3 file が揃えば通る)", []Entry{{Name: "db-postgres-go", Dir: true, Found: goLib}}, nil, 0},
		{"同じ依存先が 2 つの機能 = 警告だけ", []Entry{
			{Name: "db-postgres-go", Dir: true, Found: goLib}, {Name: "kv-postgres-go", Dir: true, Found: goLib}}, nil, 2},
		{"runtime が違えば 重複でない", []Entry{
			{Name: "authn-cookie-next", Dir: true, Found: []string{"src/interface.ts", "src/mock.ts", "src/real.ts"}},
			{Name: "authn-cookie-angular", Dir: true, Found: []string{"src/interface.ts", "src/mock.ts", "src/real.ts"}}}, nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Check(c.in, vocab(t))
			if !reflect.DeepEqual(r.Violations, c.want) {
				t.Errorf("違反 =\n%v\nwant\n%v", r.Violations, c.want)
			}
			if len(r.Warnings) != c.wantWarn {
				t.Errorf("警告 = %v (want %d 件)", r.Warnings, c.wantWarn)
			}
		})
	}
}

// TC-OUT-LINT-PLATFORM-TABLE (陽性対照)
func TestRulesTable_TC_OUT_LINT_PLATFORM_TABLE(t *testing.T) {
	v := vocab(t)
	for _, c := range CommonCapabilities {
		if slices.Contains(VagueWords, c) {
			t.Errorf("共通の表に 中身を表さない語 %s", c)
		}
		for _, r := range Runtimes {
			if _, _, _, reason := v.ParseName(c + "-x-" + r); reason != "" {
				t.Errorf("%s-x-%s が通らない: %s", c, r, reason)
			}
		}
	}
	for _, r := range Runtimes {
		if len(RequiredFiles[r]) != 3 {
			t.Errorf("runtime %s の必須の file が %d 個", r, len(RequiredFiles[r]))
		}
	}
}

// TC-OUT-LINT-PLATFORM-VOCAB
func TestVocab_TC_OUT_LINT_PLATFORM_VOCAB(t *testing.T) {
	goLib := []string{"adapter_mock.go", "adapter_real.go", "interface.go"}
	r := Check([]Entry{{Name: "pay-stripe-go", Dir: true, Found: goLib}, {Name: "sms-twilio-go", Dir: true, Found: goLib}}, vocab(t, "pay"))
	if len(r.Violations) != 1 || r.Violations[0].Name != "sms-twilio-go" {
		t.Errorf("違反 = %v (want sms-twilio-go の 1 件)", r.Violations)
	}
	for _, w := range []string{"util", "Pay", "pay-x"} {
		if _, err := NewVocab([]string{w}); err == nil {
			t.Errorf("宣言 %q が通った", w)
		}
	}
}

// TC-OUT-LINT-PLATFORM-SCAFFOLD
func TestScaffold_TC_OUT_LINT_PLATFORM_SCAFFOLD(t *testing.T) {
	v := vocab(t)
	for _, rt := range Runtimes {
		name, files, err := Scaffold(v, "db", "postgres", rt)
		if err != nil {
			t.Fatalf("%s: %v", rt, err)
		}
		var found []string
		for p := range files {
			found = append(found, p)
		}
		slices.Sort(found)
		if r := Check([]Entry{{Name: name, Dir: true, Found: found}}, v); len(r.Violations) != 0 {
			t.Errorf("%s: 作った物が検査に落ちる: %v", rt, r.Violations)
		}
		for p, body := range files {
			if !strings.Contains(body, "Port") {
				t.Errorf("%s %s に 口の名前が無い", rt, p)
			}
		}
	}
	if _, files, err := Scaffold(v, "misc", "x", "go"); err == nil || files != nil {
		t.Errorf("misc-x-go を作った (err=%v)", err)
	}
}
