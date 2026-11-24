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

// FS は memory の上の木。disk を触らない。symlink は持たない (Lstat は Stat と同じ値を返す)。
type FS struct {
	recorder
	mu    sync.Mutex
	files map[string][]byte
	dirs  map[string]bool
}

// NewFS は tree から FS を作る。
func NewFS(tree Tree) (*FS, error) {
	f := &FS{files: map[string][]byte{}, dirs: map[string]bool{".": true}}
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

func (f *FS) ReadFile(p string) ([]byte, error) {
	f.record("ReadFile", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if data, ok := f.files[c]; ok {
		return append([]byte(nil), data...), nil
	}
	if f.dirs[c] {
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
	return f.put(c, data)
}

func (f *FS) ReadDir(p string) ([]outside.DirEntry, error) {
	f.record("ReadDir", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[c]; ok {
		return nil, pathErr("readdir", c, errors.New("dir ではない"))
	}
	if !f.dirs[c] {
		return nil, pathErr("readdir", c, fs.ErrNotExist)
	}
	out := []outside.DirEntry{}
	for name := range f.files {
		if path.Dir(name) == c {
			out = append(out, outside.DirEntry{Name: path.Base(name)})
		}
	}
	for name := range f.dirs {
		if name != "." && path.Dir(name) == c {
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
	if data, ok := f.files[c]; ok {
		return outside.FileInfo{Name: path.Base(c), Size: int64(len(data))}, nil
	}
	if f.dirs[c] {
		return outside.FileInfo{Name: path.Base(c), IsDir: true}, nil
	}
	return outside.FileInfo{}, pathErr("stat", c, fs.ErrNotExist)
}

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
	if _, ok := f.files[c]; ok {
		delete(f.files, c)
		return nil
	}
	if !f.dirs[c] {
		return pathErr("remove", c, fs.ErrNotExist)
	}
	for name := range f.files {
		if path.Dir(name) == c {
			return pathErr("remove", c, errors.New("dir が空ではない"))
		}
	}
	for name := range f.dirs {
		if name != "." && path.Dir(name) == c {
			return pathErr("remove", c, errors.New("dir が空ではない"))
		}
	}
	delete(f.dirs, c)
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
	if data, ok := f.files[c]; ok {
		return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
	}
	if f.dirs[c] {
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
	old := f.files[c]
	return f.put(c, append(append([]byte(nil), old...), data...))
}

func (f *FS) MkdirAll(p string) error {
	f.record("MkdirAll", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mkdirAll(c)
}

// Lstat は Stat と同じ値を返す (mock の木に symlink は無い)。
func (f *FS) Lstat(p string) (outside.FileInfo, error) {
	f.record("Lstat", p)
	c, err := rule.CleanRel(p)
	if err != nil {
		return outside.FileInfo{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if data, ok := f.files[c]; ok {
		return outside.FileInfo{Name: path.Base(c), Size: int64(len(data))}, nil
	}
	if f.dirs[c] {
		return outside.FileInfo{Name: path.Base(c), IsDir: true}, nil
	}
	return outside.FileInfo{}, pathErr("lstat", c, fs.ErrNotExist)
}

func linkErr(from, to string, err error) error {
	return &fs.PathError{Op: "rename", Path: from + " -> " + to, Err: err}
}

// Rename は file か dir (中身ごと) を移す。to に file が在れば 置き換え、dir が在れば 誤り。
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
	data, isFile := f.files[cf]
	if !isFile && !f.dirs[cf] {
		return linkErr(cf, ct, fs.ErrNotExist)
	}
	if cf == ct {
		return nil
	}
	if !isFile && rule.Within(ct, cf) {
		return linkErr(cf, ct, errors.New("dir を 自分の下へは移せない"))
	}
	parent := path.Dir(ct)
	if _, ok := f.files[parent]; ok {
		return linkErr(cf, ct, errors.New("移し先の親が dir ではない"))
	}
	if !f.dirs[parent] {
		return linkErr(cf, ct, fs.ErrNotExist)
	}
	if f.dirs[ct] {
		return linkErr(cf, ct, errors.New("移し先に dir が在る"))
	}
	if isFile {
		delete(f.files, cf)
		f.files[ct] = data
		return nil
	}
	if _, ok := f.files[ct]; ok {
		return linkErr(cf, ct, errors.New("dir を file の上へは移せない"))
	}
	moved := func(name string) string { return ct + strings.TrimPrefix(name, cf) }
	files := map[string][]byte{}
	for name, d := range f.files {
		if rule.Within(name, cf) {
			files[moved(name)] = d
			delete(f.files, name)
		}
	}
	dirs := map[string]bool{}
	for name := range f.dirs {
		if rule.Within(name, cf) {
			dirs[moved(name)] = true
			delete(f.dirs, name)
		}
	}
	for name, d := range files {
		f.files[name] = d
	}
	for name := range dirs {
		f.dirs[name] = true
	}
	return nil
}

// RemoveAll は path を 中身ごと消す。無い path は 誤りでない。根は 誤り。
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
	for name := range f.files {
		if rule.Within(name, c) {
			delete(f.files, name)
		}
	}
	for name := range f.dirs {
		if rule.Within(name, c) {
			delete(f.dirs, name)
		}
	}
	return nil
}
