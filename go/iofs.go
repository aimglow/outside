package outside

// iofs.go — FS を io/fs の FS として読ませる (値の関数が io/fs.FS を受け、入口が 宣言した根の FS を渡す)。kifune #244 が作った口を 部品へ移した。

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"path"
	"time"
)

// IOFS は o を io/fs.FS (ReadFile・ReadDir・Stat も持つ) として返す。
func IOFS(o FS) fs.FS { return ioFS{o: o} }

type ioFS struct{ o FS }

func (f ioFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	info, err := f.o.Stat(name)
	if err != nil {
		return nil, err
	}
	if info.IsDir {
		ents, err := f.ReadDir(name)
		if err != nil {
			return nil, err
		}
		return &dir{info: fileInfo{name: path.Base(name), info: info}, ents: ents}, nil
	}
	b, err := f.o.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return &file{info: fileInfo{name: path.Base(name), info: info}, r: bytes.NewReader(b)}, nil
}

func (f ioFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}
	return f.o.ReadFile(name)
}

func (f ioFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	ents, err := f.o.ReadDir(name)
	if err != nil {
		return nil, err
	}
	out := make([]fs.DirEntry, 0, len(ents))
	for _, e := range ents {
		out = append(out, dirEntry{f: f, dir: name, e: e})
	}
	return out, nil
}

func (f ioFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	info, err := f.o.Stat(name)
	if err != nil {
		return nil, err
	}
	return fileInfo{name: path.Base(name), info: info}, nil
}

type fileInfo struct {
	name string
	info FileInfo
}

func (i fileInfo) Name() string { return i.name }
func (i fileInfo) Size() int64  { return i.info.Size }
func (i fileInfo) Mode() fs.FileMode {
	if i.info.IsDir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

// ModTime は outside の FS が時刻を持たない ∴ 零の時刻。
func (i fileInfo) ModTime() time.Time { return time.Time{} }
func (i fileInfo) IsDir() bool        { return i.info.IsDir }
func (i fileInfo) Sys() any           { return nil }

type dirEntry struct {
	f   ioFS
	dir string
	e   DirEntry
}

func (d dirEntry) Name() string { return d.e.Name }
func (d dirEntry) IsDir() bool  { return d.e.IsDir }
func (d dirEntry) Type() fs.FileMode {
	if d.e.IsDir {
		return fs.ModeDir
	}
	return 0
}
func (d dirEntry) Info() (fs.FileInfo, error) { return d.f.Stat(path.Join(d.dir, d.e.Name)) }

type file struct {
	info fileInfo
	r    *bytes.Reader
}

func (f *file) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *file) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *file) Close() error               { return nil }

type dir struct {
	info fileInfo
	ents []fs.DirEntry
	pos  int
}

func (d *dir) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *dir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.name, Err: errors.New("dir は読めない")}
}
func (d *dir) Close() error { return nil }

// ReadDir は fs.ReadDirFile の決まり (n <= 0 = 残りの全て、尽きたら n > 0 のとき io.EOF)。
func (d *dir) ReadDir(n int) ([]fs.DirEntry, error) {
	rest := d.ents[d.pos:]
	if n <= 0 {
		d.pos = len(d.ents)
		return rest, nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	if n > len(rest) {
		n = len(rest)
	}
	d.pos += n
	return rest[:n], nil
}
