package naming

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Entry は --root の直下の 1 項目。Dir が false なら file。
// Files は lib の中の file の相対 path (lib の直下からの path、例 "adapter_real.go"・"src/real.ts")。
// Subdirs は lib の直下の dir 名。
type Entry struct {
	Name    string
	Dir     bool
	Files   []string
	Subdirs []string
}

// Finding は 1 件の違反。
type Finding struct {
	Name   string
	Reason string
}

func (f Finding) String() string { return f.Name + ": " + f.Reason }

// Check は --root の直下の項目を 規則に照らし 違反を 名前の順で返す。disk を読まない。
func Check(entries []Entry) []Finding {
	var out []Finding
	for _, e := range entries {
		if !e.Dir {
			out = append(out, Finding{e.Name, "lib の dir 以外の file は置かない"})
			continue
		}
		if !namePattern.MatchString(e.Name) {
			out = append(out, Finding{e.Name, "名前が <機能>-<依存先>-<runtime> の形でない (機能 = " +
				strings.Join(Capabilities, "|") + "、runtime = " + strings.Join(Runtimes, "|") + ")"})
			continue
		}
		rt := e.Name[strings.LastIndex(e.Name, "-")+1:]
		have := map[string]bool{}
		for _, f := range e.Files {
			dir, base := path.Split(f)
			dir = strings.TrimSuffix(dir, "/")
			if dir == "" || contains(AllowedSubdirs, dir) {
				have[base] = true
			}
		}
		for _, want := range RequiredFiles[rt] {
			if !have[want] {
				out = append(out, Finding{e.Name, fmt.Sprintf("%s が無い (runtime %s の 3 file = %s)", want, rt, strings.Join(RequiredFiles[rt], "・"))})
			}
		}
		for _, d := range e.Subdirs {
			if !contains(AllowedSubdirs, d) {
				out = append(out, Finding{e.Name, fmt.Sprintf("dir %s は置かない (置いてよい dir = %s)", d, strings.Join(AllowedSubdirs, "・"))})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
