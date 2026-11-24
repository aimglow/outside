// Command naming は adapters 層 (libs/platform) の 名前と構成を検査する。
// 使い方: go run github.com/aimglow/outside/lint/naming@<tag> --root libs/platform
// exit = 0 違反なし / 1 違反あり / 3 読めない (判定不能)。
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aimglow/outside/lint/naming/naming"
)

func main() {
	root := flag.String("root", "", "検査する dir (例 libs/platform)")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "naming: --root が要る")
		os.Exit(3)
	}
	entries, err := read(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "naming: %s を読めない (判定不能): %v\n", *root, err)
		os.Exit(3)
	}
	found := naming.Check(entries)
	for _, f := range found {
		fmt.Printf("%s/%s\n", strings.TrimSuffix(*root, "/"), f)
	}
	if len(found) > 0 {
		fmt.Fprintf(os.Stderr, "naming: 違反 %d 件\n", len(found))
		os.Exit(1)
	}
	fmt.Printf("naming: %s の %d 項目 違反なし\n", *root, len(entries))
}

// read は root の直下を読み 各 lib の直下の dir と file (直下と 1 段下) を集める。
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
		if de.IsDir() {
			lib := filepath.Join(root, de.Name())
			err := filepath.WalkDir(lib, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(lib, p)
				if rel == "." {
					return nil
				}
				depth := strings.Count(rel, string(filepath.Separator))
				if d.IsDir() {
					if depth == 0 {
						e.Subdirs = append(e.Subdirs, d.Name())
					}
					if depth >= 1 {
						return fs.SkipDir
					}
					return nil
				}
				e.Files = append(e.Files, filepath.ToSlash(rel))
				return nil
			})
			if err != nil {
				return nil, err
			}
			sort.Strings(e.Subdirs)
			sort.Strings(e.Files)
		}
		out = append(out, e)
	}
	return out, nil
}
