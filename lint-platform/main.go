// Command lint-platform は adapters 層 (各 project の libs/platform) の 名前と構成を検査し 雛形を作る。
//
//	check    --root libs/platform [--vocab outside-platform.toml]
//	scaffold --root libs/platform [--vocab outside-platform.toml] <機能> <依存先> <runtime>
//	tests    --repo <repo の根> [--vocab <repo の根>/outside-platform.toml]
//
// exit = 0 違反なし (警告は出すが止めない) / 1 違反あり・作れない名前 / 3 読めない (判定不能)。
// --vocab の file が無ければ 宣言なし (共通の語だけ・adapter_dirs なし) で検査する (在るのに読めない時は 3)。
// tests は integration の印を持つ test が 置き場 (libs/platform/** と adapter_dirs) の外に在れば落とす (lint.platform.tests)。
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/aimglow/outside/lint-platform/naming"
	"github.com/aimglow/outside/lint-platform/placement"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "check" && args[0] != "scaffold" && args[0] != "tests") {
		fmt.Fprintln(stderr, "lint-platform: check か scaffold か tests を指す (使い方 = この file の冒頭)")
		return 3
	}
	fl := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fl.SetOutput(stderr)
	if args[0] == "tests" {
		repo := fl.String("repo", "", "repo の根")
		vocabPath := fl.String("vocab", "", "宣言 (既定 = <repo の根>/outside-platform.toml)")
		if err := fl.Parse(args[1:]); err != nil || *repo == "" || fl.NArg() != 0 {
			fmt.Fprintln(stderr, "lint-platform tests: --repo <repo の根> が要る (他の引数は取らない)")
			return 3
		}
		if *vocabPath == "" {
			*vocabPath = filepath.Join(*repo, "outside-platform.toml")
		}
		d, err := loadDecl(*vocabPath)
		if err != nil {
			fmt.Fprintf(stderr, "lint-platform: 宣言 %s を使えない (判定不能): %v\n", *vocabPath, err)
			return 3
		}
		return tests(*repo, d.place, stdout, stderr)
	}
	root := fl.String("root", "", "adapters 層の dir (例 libs/platform)")
	vocabPath := fl.String("vocab", "outside-platform.toml", "project の宣言 (capabilities・adapter_dirs)")
	if err := fl.Parse(args[1:]); err != nil || *root == "" {
		fmt.Fprintln(stderr, "lint-platform: --root が要る")
		return 3
	}
	d, err := loadDecl(*vocabPath)
	if err != nil {
		fmt.Fprintf(stderr, "lint-platform: 宣言 %s を使えない (判定不能): %v\n", *vocabPath, err)
		return 3
	}
	if args[0] == "scaffold" {
		return scaffold(*root, d.vocab, fl.Args(), stdout, stderr)
	}
	return check(*root, d.vocab, stdout, stderr)
}

// vocabFile は outside-platform.toml の形。書けるのは capabilities と adapter_dirs だけ。
type vocabFile struct {
	Capabilities []string `toml:"capabilities"`
	AdapterDirs  []string `toml:"adapter_dirs"`
}

// decl は 宣言を確かめた後の値 (check・scaffold は vocab、tests は place を使う)。
type decl struct {
	vocab naming.Vocab
	place placement.Decl
}

// loadDecl は 宣言の file を読む。無ければ 宣言なし。読めない・知らない key・語や dir の誤りは 誤り (判定不能)。
func loadDecl(path string) (decl, error) {
	var vf vocabFile
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return decl{}, err
	default:
		md, err := toml.Decode(string(b), &vf)
		if err != nil {
			return decl{}, err
		}
		if un := md.Undecoded(); len(un) > 0 {
			return decl{}, fmt.Errorf("知らない key %v (書けるのは capabilities と adapter_dirs だけ)", un)
		}
	}
	v, err := naming.NewVocab(vf.Capabilities)
	if err != nil {
		return decl{}, err
	}
	p, err := placement.NewDecl(vf.AdapterDirs)
	if err != nil {
		return decl{}, err
	}
	return decl{vocab: v, place: p}, nil
}

func check(root string, v naming.Vocab, stdout, stderr io.Writer) int {
	entries, err := read(root)
	if err != nil {
		fmt.Fprintf(stderr, "lint-platform: %s を読めない (判定不能): %v\n", root, err)
		return 3
	}
	r := naming.Check(entries, v)
	base := strings.TrimSuffix(root, "/")
	for _, f := range r.Warnings {
		fmt.Fprintf(stdout, "警告 %s/%s\n", base, f)
	}
	for _, f := range r.Violations {
		fmt.Fprintf(stdout, "違反 %s/%s\n", base, f)
	}
	if len(r.Violations) > 0 {
		fmt.Fprintf(stderr, "lint-platform: 違反 %d 件 / 警告 %d 件\n", len(r.Violations), len(r.Warnings))
		return 1
	}
	fmt.Fprintf(stdout, "lint-platform: %s の %d 項目 違反なし (警告 %d 件)\n", base, len(entries), len(r.Warnings))
	return 0
}

// read は 根の直下を読み 各 lib について 3 file の候補 (直下・src・lib) の在る物だけを集める。lib の中は 他を見ない。
func read(root string) ([]naming.Entry, error) {
	des, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []naming.Entry
	for _, de := range des {
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}
		e := naming.Entry{Name: de.Name(), Dir: de.IsDir()}
		if e.Dir {
			for _, files := range naming.RequiredFiles {
				for _, f := range files {
					for _, d := range naming.FileDirs {
						rel := filepath.ToSlash(filepath.Join(d, f))
						if st, err := os.Stat(filepath.Join(root, de.Name(), rel)); err == nil && !st.IsDir() && !slices.Contains(e.Found, rel) {
							e.Found = append(e.Found, rel)
						} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
							return nil, err
						}
					}
				}
			}
			slices.Sort(e.Found)
		}
		out = append(out, e)
	}
	return out, nil
}

func scaffold(root string, v naming.Vocab, rest []string, stdout, stderr io.Writer) int {
	if len(rest) != 3 {
		fmt.Fprintln(stderr, "lint-platform scaffold: <機能> <依存先> <runtime> の 3 つを渡す")
		return 1
	}
	name, files, err := naming.Scaffold(v, rest[0], rest[1], rest[2])
	if err != nil {
		fmt.Fprintf(stderr, "lint-platform scaffold: 作らない: %v\n", err)
		return 1
	}
	dir := filepath.Join(root, name)
	if _, err := os.Stat(dir); err == nil {
		fmt.Fprintf(stderr, "lint-platform scaffold: %s は既に在る (上書きしない)\n", dir)
		return 1
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			fmt.Fprintf(stderr, "lint-platform scaffold: %v\n", err)
			return 3
		}
		if err := os.WriteFile(full, []byte(files[p]), 0o644); err != nil {
			fmt.Fprintf(stderr, "lint-platform scaffold: %v\n", err)
			return 3
		}
		fmt.Fprintln(stdout, "作った", full)
	}
	return 0
}

// tests は repo の木を歩き integration の印を持つ test が 置き場の外に在れば名指す。
// 歩かない dir (placement.SkipDir) と symlink の dir は 辿らない。印を読むのに要る file が読めなければ 判定不能 (path を名乗る)。
func tests(repo string, d placement.Decl, stdout, stderr io.Writer) int {
	files, unreadable, err := walk(repo, d)
	if err != nil {
		fmt.Fprintf(stderr, "lint-platform tests: %s を歩けない (判定不能): %v\n", repo, err)
		return 3
	}
	r := placement.Check(files, d)
	unknowns := append(unreadable, r.Unknowns...)
	for _, u := range unknowns {
		fmt.Fprintf(stdout, "判定不能 %s\n", u)
	}
	for _, f := range r.Violations {
		fmt.Fprintf(stdout, "違反 %s\n", f)
	}
	if len(unknowns) > 0 {
		fmt.Fprintf(stderr, "lint-platform tests: 判定不能 %d 件 / 違反 %d 件 (読めない物が在るので 置き場の外に無いとは言えない)\n", len(unknowns), len(r.Violations))
		return 3
	}
	if len(r.Violations) > 0 {
		fmt.Fprintf(stderr, "lint-platform tests: 違反 %d 件\n", len(r.Violations))
		return 1
	}
	fmt.Fprintf(stdout, "lint-platform tests: %s の 置き場の外で 印を読んだ test %d 件 違反なし (置き場 = libs/platform/** と adapter_dirs %v)\n", repo, len(files), d.AdapterDirs())
	return 0
}

// walk は 中身を読む要る file (placement.NeedsContent) だけを読んで返す。読めない file は 判定不能として返す。
func walk(repo string, d placement.Decl) ([]placement.File, []placement.Unknown, error) {
	st, err := os.Stat(repo)
	if err != nil {
		return nil, nil, err
	}
	if !st.IsDir() {
		return nil, nil, fmt.Errorf("%s は dir でない", repo)
	}
	var files []placement.File
	var unknowns []placement.Unknown
	err = filepath.WalkDir(repo, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err // dir を読めない = 木の一部を見ていない
		}
		rel, rerr := filepath.Rel(repo, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if de.IsDir() {
			if placement.SkipDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case de.Type()&fs.ModeSymlink != 0:
			if tst, serr := os.Stat(p); serr == nil && tst.IsDir() {
				return nil // symlink の dir は 辿らない
			}
		case !de.Type().IsRegular():
			return nil
		}
		if !placement.NeedsContent(rel, d) {
			return nil
		}
		b, ferr := os.ReadFile(p)
		if ferr != nil {
			unknowns = append(unknowns, placement.Unknown{Path: rel, Reason: "読めない: " + ferr.Error()})
			return nil
		}
		files = append(files, placement.File{Path: rel, Content: b})
		return nil
	})
	return files, unknowns, err
}
