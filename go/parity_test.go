package outside_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// TC-OUT-PARITY: 部品 go の口と method の名前の組が 設計書の ports.backend_and_tools と一致する。
// 期待の表は 設計書の ports.backend_and_tools を写した物 (設計書の file は test から読まない)。
var designPorts = map[string][]string{
	"Clock":   {"Now", "Sleep"},
	"Env":     {"Lookup"},
	"FS":      {"ReadFile", "WriteFile", "ReadDir", "Stat", "Remove"},
	"Proc":    {"Run"},
	"Workdir": {"New", "Path", "Cleanup", "FS"},
	"SysInfo": {"Hostname", "HomeDir", "Pid", "IsTTY"},
	"IdGen":   {"NewID"},
}

func sorted(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}

func methodNames(t reflect.Type) []string {
	var names []string
	for i := range t.NumMethod() {
		names = append(names, t.Method(i).Name)
	}
	return sorted(names)
}

func TestParity_TC_OUT_PARITY(t *testing.T) {
	set := reflect.TypeOf(outside.Set{})
	var fields []string
	for i := range set.NumField() {
		f := set.Field(i)
		fields = append(fields, f.Name)
		want, ok := designPorts[f.Name]
		if !ok {
			t.Errorf("Set の口 %s が 設計書に無い", f.Name)
			continue
		}
		if f.Name != f.Type.Name() {
			t.Errorf("Set の field %s の型が %s (口の名前と同じにする)", f.Name, f.Type.Name())
		}
		if got := methodNames(f.Type); !reflect.DeepEqual(got, sorted(want)) {
			t.Errorf("口 %s の method = %v (設計書 = %v)", f.Name, got, sorted(want))
		}
	}
	var design []string
	for name := range designPorts {
		design = append(design, name)
	}
	if got, want := sorted(fields), sorted(design); !reflect.DeepEqual(got, want) {
		t.Errorf("Set の口 = %v (設計書 = %v)", got, want)
	}

	// Base は Set の口を全部 持つ (Workdir は 置き場を受けて口を返す関数)。
	base := reflect.TypeOf(outside.Base{})
	for name := range designPorts {
		if _, ok := base.FieldByName(name); !ok {
			t.Errorf("Base に口 %s が無い", name)
		}
	}
}

// mock は 口ごとに 1 つ在り 口を満たす。
var (
	_ outside.Clock   = (*outsidemock.Clock)(nil)
	_ outside.Env     = (*outsidemock.Env)(nil)
	_ outside.FS      = (*outsidemock.FS)(nil)
	_ outside.Proc    = (*outsidemock.Proc)(nil)
	_ outside.Workdir = (*outsidemock.Workdir)(nil)
	_ outside.SysInfo = (*outsidemock.SysInfo)(nil)
	_ outside.IdGen   = (*outsidemock.IdGen)(nil)
)
