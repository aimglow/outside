// Command lint-platform は adapters 層 (各 project の libs/platform) の 名前と構成を検査し 雛形を作る。
//
//	check    --root libs/platform [--vocab outside-platform.toml]
//	scaffold --root libs/platform [--vocab outside-platform.toml] <機能> <依存先> <runtime>
//
// exit = 0 違反なし (警告は出すが止めない) / 1 違反あり・作れない名前 / 3 読めない (判定不能)。
// --vocab の file が無ければ 共通の語だけで検査する (在るのに読めない時は 3)。
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/aimglow/outside/lint-platform/naming"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 || (args[0] != "check" && args[0] != "scaffold") {
		fmt.Fprintln(os.Stderr, "lint-platform: check か scaffold を指す (使い方 = この file の冒頭)")
		return 3
	}
	fl := flag.NewFlagSet(args[0], flag.ContinueOnError)
	root := fl.String("root", "", "adapters 層の dir (例 libs/platform)")
	vocabPath := fl.String("vocab", "outside-platform.toml", "project 固有の機能の語の宣言")
	if err := fl.Parse(args[1:]); err != nil || *root == "" {
		fmt.Fprintln(os.Stderr, "lint-platform: --root が要る")
		return 3
	}
	v, err := loadVocab(*vocabPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint-platform: 宣言 %s を使えない (判定不能): %v\n", *vocabPath, err)
		return 3
	}
	if args[0] == "scaffold" {
		return scaffold(*root, v, fl.Args())
	}
	return check(*root, v)
}

type vocabFile struct {
	Capabilities []string `toml:"capabilities"`
}

func loadVocab(path string) (naming.Vocab, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return naming.NewVocab(nil)
	}
	if err != nil {
		return naming.Vocab{}, err
	}
	var vf vocabFile
	md, err := toml.Decode(string(b), &vf)
	if err != nil {
		return naming.Vocab{}, err
	}
	if un := md.Undecoded(); len(un) > 0 {
		return naming.Vocab{}, fmt.Errorf("知らない key %v (書けるのは capabilities だけ)", un)
	}
	return naming.NewVocab(vf.Capabilities)
}

func check(root string, v naming.Vocab) int {
	entries, err := read(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint-platform: %s を読めない (判定不能): %v\n", root, err)
		return 3
	}
	r := naming.Check(entries, v)
	base := strings.TrimSuffix(root, "/")
	for _, f := range r.Warnings {
		fmt.Printf("警告 %s/%s\n", base, f)
	}
	for _, f := range r.Violations {
		fmt.Printf("違反 %s/%s\n", base, f)
	}
	if len(r.Violations) > 0 {
		fmt.Fprintf(os.Stderr, "lint-platform: 違反 %d 件 / 警告 %d 件\n", len(r.Violations), len(r.Warnings))
		return 1
	}
	fmt.Printf("lint-platform: %s の %d 項目 違反なし (警告 %d 件)\n", base, len(entries), len(r.Warnings))
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

func scaffold(root string, v naming.Vocab, rest []string) int {
	if len(rest) != 3 {
		fmt.Fprintln(os.Stderr, "lint-platform scaffold: <機能> <依存先> <runtime> の 3 つを渡す")
		return 1
	}
	name, files, err := naming.Scaffold(v, rest[0], rest[1], rest[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint-platform scaffold: 作らない: %v\n", err)
		return 1
	}
	dir := filepath.Join(root, name)
	if _, err := os.Stat(dir); err == nil {
		fmt.Fprintf(os.Stderr, "lint-platform scaffold: %s は既に在る (上書きしない)\n", dir)
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
			fmt.Fprintf(os.Stderr, "lint-platform scaffold: %v\n", err)
			return 3
		}
		if err := os.WriteFile(full, []byte(files[p]), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "lint-platform scaffold: %v\n", err)
			return 3
		}
		fmt.Println("作った", full)
	}
	return 0
}
