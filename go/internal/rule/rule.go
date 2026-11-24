// Package rule は real と mock が共に使う path の決めを持つ。外は触らない。
package rule

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// ErrOutsideRoot は 根の外を指す path を表す。
var ErrOutsideRoot = errors.New("outside: 根の外の path")

// CleanRel は 根からの相対 path を整える。絶対 path と 根の外へ出る path は 誤り。
func CleanRel(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("%w: 空の path", ErrOutsideRoot)
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) {
		return "", fmt.Errorf("%w: %s", ErrOutsideRoot, p)
	}
	c := path.Clean(p)
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", fmt.Errorf("%w: %s", ErrOutsideRoot, p)
	}
	return c, nil
}

// WorkdirName は 作業場の名前を確かめる。区切りを含む名前と . と .. は 誤り。
func WorkdirName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("outside: 作業場の名前が使えない: %q", name)
	}
	return nil
}

// Within は 整えた相対 path p が dir そのものか その下に在るかを返す。dir が "." なら 全て。
func Within(p, dir string) bool {
	return dir == "." || p == dir || strings.HasPrefix(p, dir+"/")
}
