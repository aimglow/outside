package outsidemock

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/internal/rule"
)

// Tree は mock の FS に渡す dir の木。根からの相対 path → 中身。
// 末尾 / の path は 空の dir。絶対 path と 根の外へ出る path は NewFS が誤りにする。
type Tree map[string]string

// FS は memory の上の木。disk を触らない。symlink の節 (Symlink で作る) を持つ。
// 辿る操作 (ReadFile・WriteFile・AppendFile・ReadDir・Stat・Open・MkdirAll) は 根の中に解ける target だけを辿る
// (相対の target は link の dir からの相対。絶対 path と 根の外へ出る target は ErrOutsideRoot。循環は 上限で誤り)。
// Lstat・Remove・RemoveAll・Rename・Readlink は 最後の名前の symlink を辿らず link 自身を扱う (途中の dir の symlink は辿る)。
type FS struct {
	recorder
	mu    sync.Mutex
	files map[string][]byte
	dirs  map[string]bool
	links map[string]string
}

// NewFS は tree から FS を作る。
func NewFS(tree Tree) (*FS, error) {
	f := &FS{files: map[string][]byte{}, dirs: map[string]bool{".": true}, links: map[string]string{}}
	keys := make([]string, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c, err := rule.CleanRel(k)
		if err != nil {
			return nil, fmt.Errorf("outsidemock: 木の宣言が誤り: %w", err)
		}
		if strings.HasSuffix(k, "/") {
			if tree[k] != "" {
				return nil, fmt.Errorf("outsidemock: 木の宣言が誤り: dir %s に中身", k)
			}
			if err := f.mkdirAll(c); err != nil {
				return nil, fmt.Errorf("outsidemock: 木の宣言が誤り: %w", err)
			}
			continue
		}
		if err := f.put(c, []byte(tree[k])); err != nil {
			return nil, fmt.Errorf("outsidemock: 木の宣言が誤り: %w", err)
		}
	}
	return f, nil
}

// mkdirAll は p と その親を dir にする。途中が file なら 誤り。
func (f *FS) mkdirAll(p string) error {
	for d := p; d != "."; d = path.Dir(d) {
		if _, isFile := f.files[d]; isFile {
			return pathErr("mkdir", d, errors.New("file が在る"))
		}
		if _, isLink := f.links[d]; isLink {
			return pathErr("mkdir", d, errors.New("symlink が在る"))
		}
		f.dirs[d] = true
	}
	return nil
}

// put は p に file を置く。親の dir は 無ければ作る。
func (f *FS) put(p string, data []byte) error {
	if f.dirs[p] {
		return pathErr("write", p, errors.New("dir が在る"))
	}
	if err := f.mkdirAll(path.Dir(p)); err != nil {
		return err
	}
	f.files[p] = append([]byte(nil), data...)
	return nil
}

func pathErr(op, p string, err error) error {
	return &fs.PathError{Op: op, Path: p, Err: err}
}

// maxLinkHops は 1 つの path を解く間に辿る symlink の上限 (超えたら 循環とみなし 誤り)。
const maxLinkHops = 40

// resolve は 整えた相対 path c の symlink を 根の中で解いた path (symlink を含まない) を返す。
// followLast が false なら 最後の名前は辿らない。無い名前は そのまま残す。
func (f *FS) resolve(c string, followLast bool) (string, error) {
	if c == "." {
		return ".", nil
	}
	parts := strings.Split(c, "/")
	cur := "."
	hops := 0
	for i := 0; i < len(parts); i++ {
		next := path.Join(cur, parts[i])
		target, isLink := f.links[next]
		if !isLink || (i == len(parts)-1 && !followLast) {
			cur = next
			continue
		}
		hops++
		if hops > maxLinkHops {
			return "", pathErr("resolve", c, errors.New("symlink の辿りが上限を超えた (循環)"))
		}
		if strings.HasPrefix(target, "/") {
			return "", fmt.Errorf("%w: symlink %s が 根の外 %s を指す", rule.ErrOutsideRoot, next, target)
		}
		resolved, err := rule.CleanRel(path.Join(cur, target))
		if err != nil {
			return "", fmt.Errorf("%w: symlink %s が 根の外 %s を指す", rule.ErrOutsideRoot, next, target)
		}
		// 解いた先から 残りの名前を 根から辿り直す (target の中の symlink も解く)。
		parts = append(strings.Split(resolved, "/"), parts[i+1:]...)
		cur = "."
		i = -1
	}
	return cur, nil
}

// lpath は 整えた c の 途中の symlink を解き 最後の名前は辿らない (link 自身を扱う操作の path)。
func (f *FS) lpath(c string) (string, error) {
	return f.resolve(c, false)
}

// write は c (辿る) に data を書く。appendMode なら 末尾に足す。親の dir は 無ければ作る (link の先の親は 在る事)。
func (f *FS) write(op, c string, data []byte, appendMode bool) error {
	parent, err := f.resolve(path.Dir(c), true)
	if err != nil {
		return err
	}
	if err := f.mkdirAll(parent); err != nil {
		return err
	}
	r, err := f.resolve(c, true)
	if err != nil {
		return err
	}
	if r == "." || f.dirs[r] {
		return pathErr(op, c, errors.New("dir が在る"))
	}
	if !f.dirs[path.Dir(r)] {
		return pathErr(op, c, fs.ErrNotExist)
	}
	var out []byte
	if appendMode {
		out = append(out, f.files[r]...)
	}
	f.files[r] = append(out, data...)
	return nil
}

func (f *FS) ReadFile(p string) ([]byte, error) {
	f.record("ReadFile", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.resolve(c, true)
	if err != nil {
		return nil, err
	}
	if data, ok := f.files[r]; ok {
		return append([]byte(nil), data...), nil
	}
	if f.dirs[r] {
		return nil, pathErr("read", c, errors.New("dir は読めない"))
	}
	return nil, pathErr("read", c, fs.ErrNotExist)
}

func (f *FS) WriteFile(p string, data []byte) error {
	f.record("WriteFile", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.write("write", c, data, false)
}

func (f *FS) ReadDir(p string) ([]outside.DirEntry, error) {
	f.record("ReadDir", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.resolve(c, true)
	if err != nil {
		return nil, err
	}
	if _, ok := f.files[r]; ok {
		return nil, pathErr("readdir", c, errors.New("dir ではない"))
	}
	if !f.dirs[r] {
		return nil, pathErr("readdir", c, fs.ErrNotExist)
	}
	out := []outside.DirEntry{}
	for name := range f.files {
		if path.Dir(name) == r {
			out = append(out, outside.DirEntry{Name: path.Base(name)})
		}
	}
	// symlink は 辿らずに 1 件 (IsDir = false。real の ReadDir と同じ)。
	for name := range f.links {
		if path.Dir(name) == r {
			out = append(out, outside.DirEntry{Name: path.Base(name)})
		}
	}
	for name := range f.dirs {
		if name != "." && path.Dir(name) == r {
			out = append(out, outside.DirEntry{Name: path.Base(name), IsDir: true})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *FS) Stat(p string) (outside.FileInfo, error) {
	f.record("Stat", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return outside.FileInfo{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.resolve(c, true)
	if err != nil {
		return outside.FileInfo{}, err
	}
	if data, ok := f.files[r]; ok {
		return outside.FileInfo{Name: path.Base(c), Size: int64(len(data))}, nil
	}
	if f.dirs[r] {
		return outside.FileInfo{Name: path.Base(c), IsDir: true}, nil
	}
	return outside.FileInfo{}, pathErr("stat", c, fs.ErrNotExist)
}

// hasChild は dir r の直下に 何か在るかを返す。
func (f *FS) hasChild(r string) bool {
	for name := range f.files {
		if path.Dir(name) == r {
			return true
		}
	}
	for name := range f.links {
		if path.Dir(name) == r {
			return true
		}
	}
	for name := range f.dirs {
		if name != "." && path.Dir(name) == r {
			return true
		}
	}
	return false
}

// Remove は file・symlink (辿らない)・空の dir を消す。
func (f *FS) Remove(p string) error {
	f.record("Remove", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if c == "." {
		return pathErr("remove", c, errors.New("根は消せない"))
	}
	r, err := f.lpath(c)
	if err != nil {
		return err
	}
	if _, ok := f.links[r]; ok {
		delete(f.links, r)
		return nil
	}
	if _, ok := f.files[r]; ok {
		delete(f.files, r)
		return nil
	}
	if !f.dirs[r] {
		return pathErr("remove", c, fs.ErrNotExist)
	}
	if r == "." {
		return pathErr("remove", c, errors.New("根は消せない"))
	}
	if f.hasChild(r) {
		return pathErr("remove", c, errors.New("dir が空ではない"))
	}
	delete(f.dirs, r)
	return nil
}

// Open は 中身の写しを読む stream を返す。Open の後の書き換えは 読む中身に効かない。
func (f *FS) Open(p string) (io.ReadCloser, error) {
	f.record("Open", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.resolve(c, true)
	if err != nil {
		return nil, err
	}
	if data, ok := f.files[r]; ok {
		return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
	}
	if f.dirs[r] {
		return nil, pathErr("open", c, errors.New("dir は読めない"))
	}
	return nil, pathErr("open", c, fs.ErrNotExist)
}

func (f *FS) AppendFile(p string, data []byte) error {
	f.record("AppendFile", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.write("append", c, data, true)
}

func (f *FS) MkdirAll(p string) error {
	f.record("MkdirAll", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.resolve(c, true)
	if err != nil {
		return err
	}
	return f.mkdirAll(r)
}

// Lstat は 最後の名前の symlink を辿らずに見る。symlink は IsSymlink で名乗る。
func (f *FS) Lstat(p string) (outside.FileInfo, error) {
	f.record("Lstat", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return outside.FileInfo{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.lpath(c)
	if err != nil {
		return outside.FileInfo{}, err
	}
	if _, ok := f.links[r]; ok {
		return outside.FileInfo{Name: path.Base(c), IsSymlink: true}, nil
	}
	if data, ok := f.files[r]; ok {
		return outside.FileInfo{Name: path.Base(c), Size: int64(len(data))}, nil
	}
	if f.dirs[r] {
		return outside.FileInfo{Name: path.Base(c), IsDir: true}, nil
	}
	return outside.FileInfo{}, pathErr("lstat", c, fs.ErrNotExist)
}

func linkErr(from, to string, err error) error {
	return &fs.PathError{Op: "rename", Path: from + " -> " + to, Err: err}
}

// Rename は file・symlink (辿らない)・dir (中身ごと) を移す。to に file か symlink が在れば 置き換え、dir が在れば 誤り。
func (f *FS) Rename(from, to string) error {
	f.record("Rename", from, to)
	cf, err := rule.CleanRel(from)
	if err != nil {
		return err
	}
	ct, err := rule.CleanRel(to)
	if err != nil {
		return err
	}
	if cf == "." || ct == "." {
		return linkErr(cf, ct, errors.New("根は移せない"))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	rf, err := f.lpath(cf)
	if err != nil {
		return err
	}
	rt, err := f.lpath(ct)
	if err != nil {
		return err
	}
	data, isFile := f.files[rf]
	target, isLink := f.links[rf]
	if !isFile && !isLink && !f.dirs[rf] {
		return linkErr(cf, ct, fs.ErrNotExist)
	}
	if rf == rt {
		return nil
	}
	isDir := !isFile && !isLink
	if isDir && rule.Within(rt, rf) {
		return linkErr(cf, ct, errors.New("dir を 自分の下へは移せない"))
	}
	parent := path.Dir(rt)
	if _, ok := f.files[parent]; ok {
		return linkErr(cf, ct, errors.New("移し先の親が dir ではない"))
	}
	if !f.dirs[parent] {
		return linkErr(cf, ct, fs.ErrNotExist)
	}
	if f.dirs[rt] {
		return linkErr(cf, ct, errors.New("移し先に dir が在る"))
	}
	_, dstFile := f.files[rt]
	_, dstLink := f.links[rt]
	switch {
	case isFile:
		delete(f.files, rf)
		delete(f.links, rt)
		f.files[rt] = data
		return nil
	case isLink:
		delete(f.links, rf)
		delete(f.files, rt)
		f.links[rt] = target
		return nil
	}
	if dstFile || dstLink {
		return linkErr(cf, ct, errors.New("dir を file の上へは移せない"))
	}
	moved := func(name string) string { return rt + strings.TrimPrefix(name, rf) }
	files := map[string][]byte{}
	for name, d := range f.files {
		if rule.Within(name, rf) {
			files[moved(name)] = d
			delete(f.files, name)
		}
	}
	links := map[string]string{}
	for name, tg := range f.links {
		if rule.Within(name, rf) {
			links[moved(name)] = tg
			delete(f.links, name)
		}
	}
	dirs := map[string]bool{}
	for name := range f.dirs {
		if rule.Within(name, rf) {
			dirs[moved(name)] = true
			delete(f.dirs, name)
		}
	}
	for name, d := range files {
		f.files[name] = d
	}
	for name, tg := range links {
		f.links[name] = tg
	}
	for name := range dirs {
		f.dirs[name] = true
	}
	return nil
}

// RemoveAll は path を 中身ごと消す (最後の名前の symlink は 辿らず link 自身を消す)。無い path は 誤りでない。根は 誤り。
func (f *FS) RemoveAll(p string) error {
	f.record("RemoveAll", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return err
	}
	if c == "." {
		return pathErr("removeall", c, errors.New("根は消せない"))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.lpath(c)
	if err != nil {
		return err
	}
	if r == "." {
		return pathErr("removeall", c, errors.New("根は消せない"))
	}
	for name := range f.files {
		if rule.Within(name, r) {
			delete(f.files, name)
		}
	}
	for name := range f.links {
		if rule.Within(name, r) {
			delete(f.links, name)
		}
	}
	for name := range f.dirs {
		if rule.Within(name, r) {
			delete(f.dirs, name)
		}
	}
	return nil
}

// Symlink は link に target (文字列のまま) を指す symlink を置く。link に symlink が在れば 貼り替える。
// link が file か dir なら 誤り。link の親の dir が無ければ 誤り (fs.ErrNotExist)。
func (f *FS) Symlink(target, link string) error {
	f.record("Symlink", target, link)
	c, err := rule.CleanRel(link)
	if err != nil {
		return err
	}
	if c == "." {
		return pathErr("symlink", c, errors.New("根は link にできない"))
	}
	if target == "" {
		return pathErr("symlink", c, errors.New("target が空"))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.lpath(c)
	if err != nil {
		return err
	}
	if _, ok := f.files[r]; ok {
		return pathErr("symlink", c, errors.New("link に file が在る"))
	}
	if f.dirs[r] {
		return pathErr("symlink", c, errors.New("link に dir が在る"))
	}
	parent := path.Dir(r)
	if _, ok := f.files[parent]; ok {
		return pathErr("symlink", c, errors.New("link の親が dir ではない"))
	}
	if !f.dirs[parent] {
		return pathErr("symlink", c, fs.ErrNotExist)
	}
	f.links[r] = target
	return nil
}

// Readlink は Symlink に渡した target を返す。symlink でなければ 誤り。
func (f *FS) Readlink(p string) (string, error) {
	f.record("Readlink", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.lpath(c)
	if err != nil {
		return "", err
	}
	if target, ok := f.links[r]; ok {
		return target, nil
	}
	if _, ok := f.files[r]; ok || f.dirs[r] {
		return "", pathErr("readlink", c, errors.New("symlink ではない"))
	}
	return "", pathErr("readlink", c, fs.ErrNotExist)
}
