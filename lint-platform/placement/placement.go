// Package placement は 本物の外部に触れる test (integration の印を持つ test) の置き場の規則を持つ (規則の正本)。
// 説明 = docs/developer/design.outside.yaml の lint.platform.tests (REQ-OUT-INTEGRATION-PLACE、test-classification.md §4)。
// disk を読まない。値を受けて値を返す。
package placement

import (
	"errors"
	"fmt"
	"go/build/constraint"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// PlatformDir は adapters 層の 既定の置き場 (宣言 platform_dir を書かない時)。
const PlatformDir = "libs/platform"

// OutsideDir は 取り込んだ outside の写し。部品の test は写さないので 歩かない。
const OutsideDir = "libs/outside"

// SkipDirNames は どの深さでも 歩かない dir の名前。
var SkipDirNames = []string{".git", "node_modules", ".dart_tool"}

// maxGoTags は //go:build の式の tag の数の上限。超えたら 組み合わせを数え切らず 判定不能にする。
const maxGoTags = 16

// Decl は project の宣言 (outside-platform.toml の platform_dir と adapter_dirs)。値は 確かめて揃えた形 (repo の根からの相対 path・区切りは /)。
type Decl struct {
	adapterDirs []string
	// platformDir は 宣言で書いた adapters 層の置き場 (空 = 書いていない か 置き場を持たない。noPlatform で分ける)。
	// 空の値の Decl は 既定 (libs/platform) と同じ。
	platformDir string
	// noPlatform は platform_dir = "" (置き場を持たない = adapter_dirs だけで決める)。
	noPlatform bool
	// platformDeclared は platform_dir を宣言で書いたか (書かなければ 既定 libs/platform)。
	platformDeclared bool
}

// cleanDir は 宣言の dir を確かめて揃える。空・.・絶対 path・根の外 (..) は 宣言できない (誤り = 判定不能)。
func cleanDir(key, d string) (string, error) {
	if strings.TrimSpace(d) == "" {
		return "", fmt.Errorf("%s に 空の path は書けない", key)
	}
	s := filepath.ToSlash(d)
	if path.IsAbs(s) || filepath.IsAbs(d) || filepath.VolumeName(d) != "" {
		return "", fmt.Errorf("%s の %q は 絶対 path (repo の根からの相対 path で書く)", key, d)
	}
	c := path.Clean(s)
	if c == "." {
		return "", fmt.Errorf("%s の %q は repo の根そのもの (根を宣言すると 全部が置き場になる)", key, d)
	}
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", fmt.Errorf("%s の %q は repo の根の外", key, d)
	}
	return c, nil
}

// NewDecl は adapter_dirs と platform_dir を確かめて揃える。platformDir が nil なら 既定 (libs/platform)、
// 空の文字なら 置き場を持たない (adapter_dirs だけ)、それ以外は repo の根からの相対 path (adapter_dirs と同じ確かめ)。
func NewDecl(dirs []string, platformDir *string) (Decl, error) {
	var out []string
	for _, d := range dirs {
		c, err := cleanDir("adapter_dirs", d)
		if err != nil {
			return Decl{}, err
		}
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	decl := Decl{adapterDirs: out}
	if platformDir != nil {
		decl.platformDeclared = true
		decl.noPlatform = *platformDir == ""
		if *platformDir != "" {
			c, err := cleanDir("platform_dir", *platformDir)
			if err != nil {
				return Decl{}, err
			}
			decl.platformDir = c
		}
	}
	return decl, nil
}

// PlatformDir は adapters 層の置き場 (空 = 持たない) と それを宣言で書いたかを返す。
func (d Decl) PlatformDir() (dir string, declared bool) { return d.dir(), d.platformDeclared }

// dir は 置き場 (空 = 持たない)。
func (d Decl) dir() string {
	switch {
	case d.noPlatform:
		return ""
	case d.platformDir == "":
		return PlatformDir
	default:
		return d.platformDir
	}
}

// Places は 置いてよい場所の名乗り (出力の文に使う)。
func (d Decl) Places() string {
	if d.dir() == "" {
		return fmt.Sprintf("adapter_dirs %v", d.adapterDirs)
	}
	return fmt.Sprintf("%s/** と adapter_dirs %v", d.dir(), d.adapterDirs)
}

// AdapterDirs は 揃えた宣言の写し。
func (d Decl) AdapterDirs() []string { return slices.Clone(d.adapterDirs) }

// under は rel が dir そのものか その下か (path の区切りで比べる。tools/dbgate2 は tools/dbgate の下でない)。
func under(rel, dir string) bool {
	return rel == dir || strings.HasPrefix(rel, dir+"/")
}

// Allowed は rel (repo の根からの相対 path・区切りは /) が 置いてよい場所 (platform_dir/** と adapter_dirs) の中か。
func Allowed(rel string, d Decl) bool {
	if pd := d.dir(); pd != "" && under(rel, pd) {
		return true
	}
	for _, a := range d.adapterDirs {
		if under(rel, a) {
			return true
		}
	}
	return false
}

// SkipDir は dir rel (repo の根からの相対 path) を 歩かないか。
func SkipDir(rel string) bool {
	if under(rel, OutsideDir) {
		return true
	}
	return slices.Contains(SkipDirNames, path.Base(rel))
}

// skipped は file rel の 祖先の dir に 歩かない dir が在るか。
func skipped(rel string) bool {
	for dir := path.Dir(rel); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if SkipDir(dir) {
			return true
		}
	}
	return false
}

var tsName = regexp.MustCompile(`\.integration\.(spec|test)\.(ts|tsx|js|mjs)$`)

// Candidate は 名前だけで 印を持ちうる test の file かを決め その runtime (go・ts・dart) を返す。
// ts は 名前が印そのもの。go と dart は 中身で印を読む (Marked)。
func Candidate(rel string) (runtime string, ok bool) {
	base := path.Base(rel)
	switch {
	case strings.HasSuffix(base, "_test.go"):
		return "go", true
	case tsName.MatchString(base):
		return "ts", true
	case strings.HasSuffix(base, "_test.dart"):
		return "dart", true
	}
	return "", false
}

// Marked は 中身から 印を持つかを決める (runtime は Candidate の返した物)。読み切れない時は 誤り (判定不能)。
func Marked(runtime string, content []byte) (bool, error) {
	switch runtime {
	case "ts":
		return true, nil
	case "go":
		return goMarked(content)
	case "dart":
		return dartMarked(content), nil
	}
	return false, fmt.Errorf("知らない runtime %q", runtime)
}

// goMarked は package の句より前の //go:build の式 (無ければ // +build の行の積) を読み tag integration が 肯定で効くか を返す。
// 効く = 他の tag の ある組み合わせで integration が真なら式が真・偽なら式が偽になる
// (integration && linux・unit || integration は印。!integration と integration を含まない式は 印でない)。
func goMarked(content []byte) (bool, error) {
	var expr, plus constraint.Expr
	for line := range strings.SplitSeq(string(content), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "//") {
			break // package の句か 他の物 = 印の置き場の終わり
		}
		switch {
		case constraint.IsGoBuild(t):
			if expr != nil {
				return false, errors.New("//go:build の行が 2 つ以上在る")
			}
			e, err := constraint.Parse(t)
			if err != nil {
				return false, fmt.Errorf("//go:build の式を読めない: %v", err)
			}
			expr = e
		case constraint.IsPlusBuild(t):
			e, err := constraint.Parse(t)
			if err != nil {
				return false, fmt.Errorf("// +build の行を読めない: %v", err)
			}
			if plus == nil {
				plus = e
			} else {
				plus = &constraint.AndExpr{X: plus, Y: e}
			}
		}
	}
	if expr == nil {
		expr = plus // //go:build が無い時だけ go は // +build を使う
	}
	if expr == nil {
		return false, nil
	}
	var tags []string
	expr.Eval(func(tag string) bool {
		if tag != "integration" && !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
		return false
	})
	if len(tags) > maxGoTags {
		return false, fmt.Errorf("//go:build の式の tag が %d 個 (上限 %d) で 組み合わせを数え切らない", len(tags), maxGoTags)
	}
	for bits := 0; bits < 1<<len(tags); bits++ {
		eval := func(integration bool) bool {
			return expr.Eval(func(tag string) bool {
				if tag == "integration" {
					return integration
				}
				return bits&(1<<slices.Index(tags, tag)) != 0
			})
		}
		if eval(true) && !eval(false) {
			return true, nil
		}
	}
	return false, nil
}

var (
	dartTagsAnno = regexp.MustCompile(`@Tags\s*\(\s*(?:const\s*)?\[[^\]]*['"]integration['"][^\]]*\]`)
	dartTagsArg  = regexp.MustCompile(`tags\s*:\s*(?:(?:const\s*)?\[[^\]]*['"]integration['"][^\]]*\]|['"]integration['"])`)
)

// dartMarked は @Tags([... 'integration' ...]) か tags: に 'integration' (または "integration") を持つか。
func dartMarked(content []byte) bool {
	return dartTagsAnno.Match(content) || dartTagsArg.Match(content)
}

// File は 検査に渡す 1 件。Path は repo の根からの相対 path (区切りは /)。
type File struct {
	Path    string
	Content []byte
}

// Finding は 置き場の外に在る 本物の外部に触れる test の 1 件。
// Name が空 = integration の印 (marks)。Name が在る = 本物の口を使う所 (real_use、Line はその行)。
type Finding struct {
	Path    string
	Line    int
	Runtime string
	Name    string
}

func (f Finding) String() string {
	if f.Name == "" {
		return f.Path + ": " + f.Runtime + " の integration の test は 置き場 (platform_dir と adapter_dirs) の外 (test-classification.md §4)"
	}
	return fmt.Sprintf("%s:%d: %s の test が 本物の口 %s を使う (置き場の外。test-classification.md §4)", f.Path, f.Line, f.Runtime, f.Name)
}

// Unknown は 読み切れなかった 1 件 (判定不能)。
type Unknown struct {
	Path   string
	Reason string
}

func (u Unknown) String() string { return u.Path + ": " + u.Reason }

// Result は 違反 (置き場の外の 本物の外部に触れる test) と 判定不能。
type Result struct {
	Violations []Finding
	Unknowns   []Unknown
}

// Check は file の一覧を 2 つの判定 (marks・real_use) に掛ける。
// files = 置き場の外の test の file と 置き場の中の adapters の本物の file と go.mod・pubspec.yaml (他の file は 見ずに飛ばす)。
// 歩かない dir の下・置き場の中の test は 見ない。結果は path・行・名前の順。
func Check(files []File, d Decl) Result {
	a, unknowns := collectAdapters(files, d)
	r := Result{Unknowns: unknowns}
	for _, f := range files {
		rel := path.Clean(filepath.ToSlash(f.Path))
		if skipped(rel) || Allowed(rel, d) {
			continue
		}
		if rt, ok := Candidate(rel); ok {
			marked, err := Marked(rt, f.Content)
			if err != nil {
				r.Unknowns = append(r.Unknowns, Unknown{rel, err.Error()})
			} else if marked {
				r.Violations = append(r.Violations, Finding{Path: rel, Runtime: rt})
			}
		}
		rt, ok := TestFile(rel)
		if !ok {
			continue
		}
		switch rt {
		case "go":
			fs, err := goRealUse(rel, f.Content, a)
			if err != nil {
				r.Unknowns = append(r.Unknowns, Unknown{rel, "Go として読めない: " + err.Error()})
			}
			r.Violations = append(r.Violations, fs...)
		case "ts":
			r.Violations = append(r.Violations, tsRealUse(rel, f.Content, a)...)
		case "dart":
			r.Violations = append(r.Violations, dartRealUse(rel, f.Content, a)...)
		}
	}
	slices.SortFunc(r.Violations, func(a, b Finding) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.Name, b.Name)
	})
	r.Violations = slices.Compact(r.Violations)
	slices.SortFunc(r.Unknowns, func(a, b Unknown) int { return strings.Compare(a.Path+a.Reason, b.Path+b.Reason) })
	r.Unknowns = slices.Compact(r.Unknowns)
	return r
}
