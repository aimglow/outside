package placement

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// 判定 2 (real_use): 置き場の外の test が 本物の口を作る物 (outside の real・adapters の real) を使うか。印に依らない。

var tsTestName = regexp.MustCompile(`\.(spec|test)\.(ts|tsx|js|mjs)$`)

// TestFile は 名前から test の file かを決め その runtime (go・ts・dart) を返す (real_use の対象)。
func TestFile(rel string) (runtime string, ok bool) {
	base := path.Base(rel)
	switch {
	case strings.HasSuffix(base, "_test.go"):
		return "go", true
	case tsTestName.MatchString(base):
		return "ts", true
	case strings.HasSuffix(base, "_test.dart"):
		return "dart", true
	}
	return "", false
}

// adapterReal は rel が adapters の本物の file の名前か。runtime を返す。
func adapterReal(rel string) (string, bool) {
	base := path.Base(rel)
	for rt, name := range AdapterRealFiles {
		if base == name {
			return rt, true
		}
	}
	return "", false
}

// manifest は go.mod・pubspec.yaml (import path と package の名前を決める材料)。
func manifest(rel string) bool {
	b := path.Base(rel)
	return b == "go.mod" || b == "pubspec.yaml"
}

// NeedsContent は main が 中身を読む要る file か:
// 置き場の外の test の file・置き場の中の adapters の本物の file・go.mod と pubspec.yaml (歩かない dir の下は除く)。
func NeedsContent(rel string, d Decl) bool {
	if skipped(rel) {
		return false
	}
	if manifest(rel) {
		return true
	}
	if Allowed(rel, d) {
		_, ok := adapterReal(rel)
		return ok
	}
	_, ok := TestFile(rel)
	return ok
}

// ModulePath は go.mod の module の path を返す (無ければ "")。
func ModulePath(gomod []byte) string {
	for line := range strings.SplitSeq(string(gomod), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == "module" {
			if s, err := strconv.Unquote(f[1]); err == nil {
				return s
			}
			return f[1]
		}
	}
	return ""
}

var pubspecName = regexp.MustCompile(`(?m)^name:\s*['"]?([A-Za-z0-9_]+)['"]?\s*$`)

// PubspecName は pubspec.yaml の name を返す (無ければ "")。
func PubspecName(pubspec []byte) string {
	if m := pubspecName.FindSubmatch(pubspec); m != nil {
		return string(m[1])
	}
	return ""
}

type goPkg struct {
	name  string
	names []string
}

type tsLib struct {
	dir, base string
	names     []string
}

type dartLib struct {
	dir, base, pkg string
	names          []string
}

type adapters struct {
	goPkgs map[string]*goPkg // import path → package
	ts     []tsLib
	dart   []dartLib
}

// nearest は dir から根へ上り manifests に在る最初の dir を返す。
func nearest(dir string, manifests map[string]string) (string, bool) {
	for {
		if _, ok := manifests[dir]; ok {
			return dir, true
		}
		if dir == "." {
			return "", false
		}
		dir = path.Dir(dir)
	}
}

// libDir は 本物の file の dir から lib の dir を決める (src・lib の下なら その親)。
func libDir(dir string) string {
	for {
		b := path.Base(dir)
		if (b == "src" || b == "lib") && dir != b {
			dir = path.Dir(dir)
			continue
		}
		return dir
	}
}

func collectAdapters(files []File, d Decl) (adapters, []Unknown) {
	mods, pubs := map[string]string{}, map[string]string{}
	for _, f := range files {
		rel := path.Clean(f.Path)
		if skipped(rel) {
			continue
		}
		switch path.Base(rel) {
		case "go.mod":
			mods[path.Dir(rel)] = ModulePath(f.Content)
		case "pubspec.yaml":
			pubs[path.Dir(rel)] = PubspecName(f.Content)
		}
	}
	a := adapters{goPkgs: map[string]*goPkg{}}
	var unknowns []Unknown
	for _, f := range files {
		rel := path.Clean(f.Path)
		if skipped(rel) || !Allowed(rel, d) {
			continue
		}
		rt, ok := adapterReal(rel)
		if !ok {
			continue
		}
		dir := path.Dir(rel)
		switch rt {
		case "go":
			modDir, ok := nearest(dir, mods)
			if !ok || mods[modDir] == "" {
				unknowns = append(unknowns, Unknown{rel, "go.mod (module の行) が 根までに無く import path を決められない"})
				continue
			}
			imp := mods[modDir]
			if dir != modDir {
				r := strings.TrimPrefix(dir, modDir+"/")
				if modDir == "." {
					r = dir
				}
				imp += "/" + r
			}
			name, names, err := goExported(rel, f.Content)
			if err != nil {
				unknowns = append(unknowns, Unknown{rel, "Go として読めない: " + err.Error()})
				continue
			}
			p := a.goPkgs[imp]
			if p == nil {
				p = &goPkg{name: name}
				a.goPkgs[imp] = p
			}
			p.names = append(p.names, names...)
		case "ts":
			ld := libDir(dir)
			a.ts = append(a.ts, tsLib{dir: ld, base: path.Base(ld), names: tsExported(f.Content)})
		case "dart":
			ld := libDir(dir)
			pkg := strings.ReplaceAll(path.Base(ld), "-", "_")
			if pd, ok := nearest(dir, pubs); ok && Allowed(pd, d) {
				ld = pd
				if n := pubs[pd]; n != "" {
					pkg = n
				}
			}
			a.dart = append(a.dart, dartLib{dir: ld, base: path.Base(ld), pkg: pkg, names: dartDeclared(f.Content)})
		}
	}
	return a, unknowns
}

// goExported は Go の file の package の名前と 公開の top level の名前 (型・関数・変数・定数) を返す。
func goExported(rel string, src []byte) (string, []string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), rel, src, parser.SkipObjectResolution)
	if err != nil {
		return "", nil, err
	}
	var names []string
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.IsExported() {
				names = append(names, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						names = append(names, s.Name.Name)
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.IsExported() {
							names = append(names, n.Name)
						}
					}
				}
			}
		}
	}
	return f.Name.Name, names, nil
}

// goRealUse は Go の test が outside の本物か adapters の本物を selector (dot import は 名前) で使う所を返す。
func goRealUse(rel string, src []byte, a adapters) ([]Finding, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	type pkg struct {
		display string
		names   []string
	}
	locals := map[string]pkg{}
	var dots []pkg
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		var k pkg
		var def string
		switch {
		case p == OutsideGoImport:
			k, def = pkg{OutsideGoPackage, OutsideGoReal}, OutsideGoPackage
		case a.goPkgs[p] != nil:
			k, def = pkg{a.goPkgs[p].name, a.goPkgs[p].names}, a.goPkgs[p].name
		default:
			continue
		}
		local := def
		if imp.Name != nil {
			local = imp.Name.Name
		}
		switch local {
		case "_":
		case ".":
			dots = append(dots, k)
		default:
			locals[local] = k
		}
	}
	if len(locals) == 0 && len(dots) == 0 {
		return nil, nil
	}
	var out []Finding
	add := func(pos token.Pos, name string) {
		out = append(out, Finding{Path: rel, Line: fset.Position(pos).Line, Runtime: "go", Name: name})
	}
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if x, ok := n.X.(*ast.Ident); ok {
				if k, ok := locals[x.Name]; ok && slices.Contains(k.names, n.Sel.Name) {
					add(n.Sel.Pos(), k.display+"."+n.Sel.Name)
				}
			}
			ast.Inspect(n.X, visit)
			return false // Sel は 名前の参照でない
		case *ast.Ident:
			for _, k := range dots {
				if slices.Contains(k.names, n.Name) {
					add(n.Pos(), k.display+"."+n.Name)
				}
			}
		case *ast.ImportSpec:
			return false
		}
		return true
	}
	for _, decl := range f.Decls {
		ast.Inspect(decl, visit)
	}
	return out, nil
}

var (
	tsImport     = regexp.MustCompile(`(?m)^[ \t]*(import|export)\s+(type\s+)?([^'";]*?)\s*from\s*['"]([^'"]+)['"]`)
	tsBareImport = regexp.MustCompile(`(?m)^[ \t]*import\s*['"]([^'"]+)['"]`)
	tsNamespace  = regexp.MustCompile(`\*\s*as\s+([A-Za-z_$][\w$]*)`)
	tsBraces     = regexp.MustCompile(`\{([^}]*)\}`)
	tsExportDecl = regexp.MustCompile(`(?m)^[ \t]*export\s+(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(?:async\s+)?(?:function\*?|class|const|let|var|enum)\s+([A-Za-z_$][\w$]*)`)
	tsExportList = regexp.MustCompile(`(?m)^[ \t]*export\s*\{([^}]*)\}`)
	tsIdent      = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
)

// tsExported は real.ts が export する 値の名前 (関数・class・変数・enum。型は除く)。
func tsExported(src []byte) []string {
	var names []string
	for _, m := range tsExportDecl.FindAllSubmatch(src, -1) {
		names = append(names, string(m[1]))
	}
	for _, m := range tsExportList.FindAllSubmatch(src, -1) {
		for el := range strings.SplitSeq(string(m[1]), ",") {
			f := strings.Fields(el)
			if len(f) == 0 || f[0] == "type" {
				continue
			}
			names = append(names, f[len(f)-1]) // a as b なら b
		}
	}
	return names
}

func lineOf(src []byte, off int) int { return strings.Count(string(src[:off]), "\n") + 1 }

// resolves は test の dir からの相対 specifier が dir の下を指すか。
func resolves(testRel, spec, dir string) bool {
	if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
		return false
	}
	return under(path.Join(path.Dir(testRel), spec), dir)
}

func hasSegment(spec, seg string) bool { return slices.Contains(strings.Split(spec, "/"), seg) }

func isOutsideTS(testRel, spec string) bool {
	for _, p := range OutsideTSPackages {
		if spec == p || strings.HasPrefix(spec, p+"/") {
			return true
		}
	}
	for _, p := range OutsideTSPlaces {
		if resolves(testRel, spec, p) || strings.Contains("/"+spec+"/", "/"+p+"/") {
			return true
		}
	}
	return false
}

// tsRealUse は TS の test の import から 本物の口を使う所を返す。
func tsRealUse(rel string, src []byte, a adapters) []Finding {
	var out []Finding
	add := func(off int, name string) {
		out = append(out, Finding{Path: rel, Line: lineOf(src, off), Runtime: "ts", Name: name})
	}
	type named struct {
		orig string
		off  int
	}
	check := func(spec string, specOff int, typeOnly bool, names []named, ns string) {
		if typeOnly {
			return
		}
		if b := path.Base(spec); b == "real" || b == "real.ts" {
			add(specOff, spec)
			return
		}
		match := func(pred func(string) bool, display func(string) string) {
			for _, n := range names {
				if pred(n.orig) {
					add(n.off, display(n.orig))
				}
			}
			if ns != "" {
				re := regexp.MustCompile(`\b` + regexp.QuoteMeta(ns) + `\s*\.\s*([A-Za-z_$][\w$]*)`)
				for _, m := range re.FindAllSubmatchIndex(src, -1) {
					if name := string(src[m[2]:m[3]]); pred(name) {
						add(m[2], display(name))
					}
				}
			}
		}
		if isOutsideTS(rel, spec) {
			match(func(n string) bool { return strings.HasPrefix(n, OutsideTSRealPrefix) }, func(n string) string { return n })
			return
		}
		for _, l := range a.ts {
			if resolves(rel, spec, l.dir) || (!strings.HasPrefix(spec, ".") && hasSegment(spec, l.base)) {
				match(func(n string) bool { return slices.Contains(l.names, n) }, func(n string) string { return l.base + "." + n })
			}
		}
	}
	for _, m := range tsImport.FindAllSubmatchIndex(src, -1) {
		typeOnly := m[4] >= 0
		clause, clauseOff := string(src[m[6]:m[7]]), m[6]
		spec := string(src[m[8]:m[9]])
		var names []named
		var ns string
		if nm := tsNamespace.FindStringSubmatch(clause); nm != nil {
			ns = nm[1]
		}
		if bm := tsBraces.FindStringSubmatchIndex(clause); bm != nil {
			body, bodyOff := clause[bm[2]:bm[3]], clauseOff+bm[2]
			pos := 0
			for el := range strings.SplitSeq(body, ",") {
				elOff := bodyOff + pos
				pos += len(el) + 1
				f := strings.Fields(el)
				if len(f) == 0 || f[0] == "type" || !tsIdent.MatchString(f[0]) {
					continue
				}
				names = append(names, named{f[0], elOff + strings.Index(el, f[0])})
			}
		}
		check(spec, m[8], typeOnly, names, ns)
	}
	for _, m := range tsBareImport.FindAllSubmatchIndex(src, -1) {
		check(string(src[m[2]:m[3]]), m[2], false, nil, "")
	}
	return out
}

var (
	dartImport   = regexp.MustCompile(`(?m)^[ \t]*import\s+['"]([^'"]+)['"]`)
	dartClass    = regexp.MustCompile(`(?m)^[ \t]*(?:(?:abstract|final|base|sealed|interface|mixin)\s+)*class\s+([A-Za-z]\w*)`)
	dartTopFunc  = regexp.MustCompile(`(?m)^(?:[A-Za-z_][\w<>?,\[\] ]*\s+)?([A-Za-z]\w*)\s*\([^;{]*\)\s*(?:async\*?|sync\*)?\s*(?:\{|=>)`)
	dartComment  = regexp.MustCompile(`//[^\n]*`)
	dartKeywords = []string{"if", "for", "while", "switch", "return", "catch", "assert", "class", "import", "export", "part", "library"}
)

// dartDeclared は adapter_real.dart が宣言する class と top level の関数の名前 (_ で始まる物は除く)。
func dartDeclared(src []byte) []string {
	var names []string
	for _, m := range dartClass.FindAllSubmatch(src, -1) {
		names = append(names, string(m[1]))
	}
	for _, m := range dartTopFunc.FindAllSubmatch(src, -1) {
		if n := string(m[1]); !slices.Contains(dartKeywords, n) && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	return names
}

// blank は 行の注釈を 同じ長さの空白にする (offset を保つ)。
func blank(src []byte, re *regexp.Regexp) []byte {
	out := slices.Clone(src)
	for _, m := range re.FindAllIndex(out, -1) {
		for i := m[0]; i < m[1]; i++ {
			out[i] = ' '
		}
	}
	return out
}

// dartRealUse は Dart の test の import と 呼び出しから 本物の口を使う所を返す。
func dartRealUse(rel string, src []byte, a adapters) []Finding {
	var out []Finding
	add := func(off int, name string) {
		out = append(out, Finding{Path: rel, Line: lineOf(src, off), Runtime: "dart", Name: name})
	}
	code := blank(blank(src, dartComment), dartImport)
	imports := dartImport.FindAllSubmatchIndex(src, -1)
	usedOutside := false
	var libs []dartLib
	for _, m := range imports {
		uri := string(src[m[2]:m[3]])
		if strings.HasPrefix(uri, OutsideDartImportPrefix) {
			usedOutside = true
			continue
		}
		if strings.HasSuffix(uri, AdapterRealFiles["dart"]) {
			add(m[2], uri)
			continue
		}
		for _, l := range a.dart {
			if strings.HasPrefix(uri, "package:"+l.pkg+"/") || resolves(rel, uri, l.dir) ||
				(!strings.Contains(uri, ":") && hasSegment(uri, l.base)) {
				libs = append(libs, l)
			}
		}
	}
	if usedOutside {
		for _, m := range outsideDartReal.FindAllSubmatchIndex(code, -1) {
			if m[2] >= 0 {
				add(m[2], string(code[m[2]:m[3]]))
			} else {
				add(m[4], "OutsideSet.real")
			}
		}
	}
	for _, l := range libs {
		for _, n := range l.names {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(n) + `\s*(?:<[^<>()]*>)?\s*[(.]`)
			for _, m := range re.FindAllIndex(code, -1) {
				add(m[0], l.base+"."+n)
			}
		}
	}
	return out
}
