package naming

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Entry は 根の直下の 1 項目。Found は lib の中で見つかった 3 file の候補の相対 path (例 "src/real.ts")。
type Entry struct {
	Name  string
	Dir   bool
	Found []string
}

// Finding は 1 件の名指し。
type Finding struct {
	Name   string
	Reason string
}

func (f Finding) String() string { return f.Name + ": " + f.Reason }

// Result は 違反 (止める) と 警告 (止めない)。
type Result struct {
	Violations []Finding
	Warnings   []Finding
}

var word = regexp.MustCompile(`^[a-z0-9]+$`)

// Vocab は 使ってよい機能の語 (共通の表 + project の宣言)。
type Vocab struct{ caps []string }

// NewVocab は project の宣言の語を 確かめて 共通の表に足す。語の形が違う・中身を表さない語は 誤り。
func NewVocab(declared []string) (Vocab, error) {
	caps := slices.Clone(CommonCapabilities)
	for _, w := range declared {
		if !word.MatchString(w) {
			return Vocab{}, fmt.Errorf("機能の語 %q は 英小文字と数字だけの 1 語で書く", w)
		}
		if slices.Contains(VagueWords, w) {
			return Vocab{}, fmt.Errorf("機能の語 %q は 中身を表さない語なので 宣言できない (%s)", w, strings.Join(VagueWords, "・"))
		}
		if !slices.Contains(caps, w) {
			caps = append(caps, w)
		}
	}
	slices.Sort(caps)
	return Vocab{caps: caps}, nil
}

// ParseName は lib の名前を <機能>-<依存先>-<runtime> に分ける。合わなければ 理由を返す。
func (v Vocab) ParseName(name string) (capability, dep, runtime string, reason string) {
	parts := strings.Split(name, "-")
	bad := "名前が <機能>-<依存先>-<runtime> の形でない (機能 = " + strings.Join(v.caps, "|") +
		"、runtime = " + strings.Join(Runtimes, "|") + "。project 固有の機能は outside-platform.toml に宣言する)"
	if len(parts) < 3 {
		return "", "", "", bad
	}
	for _, p := range parts {
		if !word.MatchString(p) {
			return "", "", "", bad
		}
	}
	capability, runtime = parts[0], parts[len(parts)-1]
	dep = strings.Join(parts[1:len(parts)-1], "-")
	if !slices.Contains(v.caps, capability) || !slices.Contains(Runtimes, runtime) {
		return "", "", "", bad
	}
	return capability, dep, runtime, ""
}

// Check は 根の直下の項目を 規則に照らす。disk を読まない。結果は 名前の順。
func Check(entries []Entry, v Vocab) Result {
	var r Result
	type key struct{ dep, runtime string }
	seen := map[key][]string{}
	for _, e := range entries {
		if !e.Dir {
			r.Violations = append(r.Violations, Finding{e.Name, "lib の dir 以外の物は 直下に置かない"})
			continue
		}
		capability, dep, rt, reason := v.ParseName(e.Name)
		if reason != "" {
			r.Violations = append(r.Violations, Finding{e.Name, reason})
			continue
		}
		seen[key{dep, rt}] = append(seen[key{dep, rt}], capability+"-"+dep+"-"+rt)
		for _, want := range RequiredFiles[rt] {
			if !hasFile(e.Found, want) {
				r.Violations = append(r.Violations, Finding{e.Name, fmt.Sprintf(
					"%s が無い (runtime %s の 3 file = %s を 直下か src/ か lib/ に置く)", want, rt, strings.Join(RequiredFiles[rt], "・"))})
			}
		}
	}
	for _, names := range seen {
		if len(names) > 1 {
			slices.Sort(names)
			for _, n := range names {
				r.Warnings = append(r.Warnings, Finding{n, "同じ依存先が 2 つ以上の機能に在る (" + strings.Join(names, "・") + ")。正当かは 設計の review で決める"})
			}
		}
	}
	byName := func(a, b Finding) int { return strings.Compare(a.Name+a.Reason, b.Name+b.Reason) }
	slices.SortStableFunc(r.Violations, byName)
	slices.SortStableFunc(r.Warnings, byName)
	return r
}

func hasFile(found []string, want string) bool {
	for _, d := range FileDirs {
		p := want
		if d != "" {
			p = d + "/" + want
		}
		if slices.Contains(found, p) {
			return true
		}
	}
	return false
}
