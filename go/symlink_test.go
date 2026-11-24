package outside_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// TC-OUT-FS-SYMLINK: Symlink・Readlink が real と mock で同じ振る舞いをする。f は 空の根から始まる。abs は 根の外の絶対 path。
func fsSymlinkContract(t *testing.T, f outside.FS, abs string) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	read := func(p, want string) {
		t.Helper()
		if got, err := f.ReadFile(p); err != nil || string(got) != want {
			t.Errorf("ReadFile(%s) = %q, %v (want %q)", p, got, err, want)
		}
	}
	readlink := func(p, want string) {
		t.Helper()
		if got, err := f.Readlink(p); err != nil || got != want {
			t.Errorf("Readlink(%s) = %q, %v (want %q)", p, got, err, want)
		}
	}
	must(f.WriteFile("t/a.txt", []byte("A")))
	must(f.WriteFile("t/b.txt", []byte("BB")))
	must(f.MkdirAll("d"))

	// 根の中の file を指す link を作る。Lstat は 辿らず名乗り Stat・ReadFile・Open は 辿る。
	must(f.Symlink("t/a.txt", "l"))
	if info, err := f.Lstat("l"); err != nil || info != (outside.FileInfo{Name: "l", IsSymlink: true}) {
		t.Errorf("Lstat(l) = %+v, %v", info, err)
	}
	readlink("l", "t/a.txt")
	read("l", "A")
	if info, err := f.Stat("l"); err != nil || info != (outside.FileInfo{Name: "l", Size: 1}) {
		t.Errorf("Stat(l) = %+v, %v (辿った先を返す)", info, err)
	}

	// 同じ link を 別の target へ貼り替える。
	must(f.Symlink("t/b.txt", "l"))
	readlink("l", "t/b.txt")
	read("l", "BB")

	// 相対の target は link の dir からの相対。dir を指す link の下も辿る。
	must(f.Symlink("../t/a.txt", "d/up"))
	read("d/up", "A")
	must(f.Symlink("t", "tl"))
	read("tl/a.txt", "A")
	if got, err := f.ReadDir("tl"); err != nil || !reflect.DeepEqual(got, []outside.DirEntry{{Name: "a.txt"}, {Name: "b.txt"}}) {
		t.Errorf("ReadDir(tl) = %+v, %v", got, err)
	}
	// 一覧の symlink は 辿らない 1 件 (IsDir = false)。貼り替えの一時の名前は 残らない。
	if got, err := f.ReadDir("."); err != nil || !reflect.DeepEqual(got, []outside.DirEntry{
		{Name: "d", IsDir: true}, {Name: "l"}, {Name: "t", IsDir: true}, {Name: "tl"},
	}) {
		t.Errorf("ReadDir(.) = %+v, %v", got, err)
	}

	// link 越しに書くと 指す先が変わり link は symlink のまま。
	must(f.WriteFile("l", []byte("B2")))
	read("t/b.txt", "B2")
	if info, err := f.Lstat("l"); err != nil || !info.IsSymlink {
		t.Errorf("link 越しに書いた後の Lstat(l) = %+v, %v", info, err)
	}

	// 宛先の link が file・dir は 誤り (中身は変わらない)。親の dir が無いのも 誤り。
	if err := f.Symlink("t/b.txt", "t/a.txt"); err == nil {
		t.Error("file の上の Symlink が通った")
	}
	read("t/a.txt", "A")
	if err := f.Symlink("t/b.txt", "d"); err == nil {
		t.Error("dir の上の Symlink が通った")
	}
	if err := f.Symlink("t/a.txt", "nope/l"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("親の dir が無い Symlink = %v", err)
	}
	if err := f.Symlink("", "empty"); err == nil {
		t.Error("空の target の Symlink が通った")
	}

	// 根の外を指す target は 作れ Readlink は target を返すが 辿る操作は 誤り。
	must(f.Symlink("../x", "out1"))
	must(f.Symlink(abs, "out2"))
	readlink("out1", "../x")
	readlink("out2", abs)
	for _, p := range []string{"out1", "out2"} {
		if got, err := f.ReadFile(p); err == nil {
			t.Errorf("根の外を指す %s の ReadFile が通った: %q", p, got)
		}
		if err := f.WriteFile(p, []byte("x")); err == nil {
			t.Errorf("根の外を指す %s の WriteFile が通った", p)
		}
		if _, err := f.Stat(p); err == nil {
			t.Errorf("根の外を指す %s の Stat が通った", p)
		}
		if info, err := f.Lstat(p); err != nil || !info.IsSymlink {
			t.Errorf("根の外を指す %s の Lstat = %+v, %v", p, info, err)
		}
	}

	// 指す先が無い link: Stat は 無い、Lstat は symlink。
	must(f.Symlink("t/none", "dang"))
	if _, err := f.Stat("dang"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("指す先が無い Stat = %v", err)
	}
	// 循環は 誤り。
	must(f.Symlink("c2", "c1"))
	must(f.Symlink("c1", "c2"))
	if _, err := f.ReadFile("c1"); err == nil {
		t.Error("循環する link の ReadFile が通った")
	}

	// symlink でない path の Readlink は 誤り。無い path は 無い。
	for _, p := range []string{"t/a.txt", "d"} {
		if got, err := f.Readlink(p); err == nil {
			t.Errorf("symlink でない %s の Readlink = %q", p, got)
		}
	}
	if _, err := f.Readlink("nope"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("無い path の Readlink = %v", err)
	}

	// Remove・RemoveAll・Rename は link 自身を扱う (指す先は そのまま)。
	must(f.Remove("l"))
	if _, err := f.Lstat("l"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Remove の後に link が残った: %v", err)
	}
	read("t/b.txt", "B2")
	must(f.RemoveAll("tl"))
	read("t/a.txt", "A")
	must(f.Rename("d/up", "up2"))
	readlink("up2", "../t/a.txt")
	if _, err := f.Lstat("d/up"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Rename の移す元が残った: %v", err)
	}

	// 根の外の link の path は 誤り。
	for _, p := range []string{"..", "../x", abs, ""} {
		if err := f.Symlink("t/a.txt", p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("Symlink(_, %q) = %v", p, err)
		}
		if _, err := f.Readlink(p); !errors.Is(err, outside.ErrOutsideRoot) {
			t.Errorf("Readlink(%q) = %v", p, err)
		}
	}
	if err := f.Symlink("t/a.txt", "."); err == nil {
		t.Error("根の Symlink が通った")
	}
}

func TestContract_TC_OUT_FS_SYMLINK(t *testing.T) {
	newAbs := func(t *testing.T) string {
		abs := filepath.Join(testDir(t), "outside.txt")
		if err := os.WriteFile(abs, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		return abs
	}
	checkAbs := func(t *testing.T, abs string) {
		if data, err := os.ReadFile(abs); err != nil || string(data) != "keep" {
			t.Errorf("根の外の file が変わった: %q, %v", data, err)
		}
		if data, err := os.ReadFile(filepath.Join(filepath.Dir(abs), "..", "x")); err == nil {
			t.Errorf("根の外に file が出来た: %q", data)
		}
	}
	t.Run("real", func(t *testing.T) {
		f, err := outside.NewFS(testDir(t))
		if err != nil {
			t.Fatal(err)
		}
		abs := newAbs(t)
		fsSymlinkContract(t, f, abs)
		checkAbs(t, abs)
	})
	t.Run("mock", func(t *testing.T) {
		f, err := outsidemock.NewFS(nil)
		if err != nil {
			t.Fatal(err)
		}
		abs := newAbs(t)
		fsSymlinkContract(t, f, abs)
		checkAbs(t, abs)
		want := []outsidemock.Call{{Method: "Symlink", Args: []string{"t/a.txt", "l"}}}
		if got := outsidemock.CallsOf(f.Calls(), "Symlink")[:1]; !reflect.DeepEqual(got, want) {
			t.Errorf("Symlink の記録 = %+v", got)
		}
		if got := outsidemock.CallsOf(f.Calls(), "Readlink"); len(got) == 0 || !reflect.DeepEqual(got[0].Args, []string{"l"}) {
			t.Errorf("Readlink の記録 = %+v", got)
		}
	})
	// limitFS も link の path を 宣言した根で確かめる (target は 文字列ゆえ 確かめない)。
	t.Run("limit", func(t *testing.T) {
		inner, err := outsidemock.NewFS(outsidemock.Tree{"in/a.txt": "A", "other/b.txt": "B"})
		if err != nil {
			t.Fatal(err)
		}
		var got error
		err = outside.RunToolWith(outside.Decl{FSRoots: []string{"in"}}, outside.Base{FS: inner}, func(s outside.Set) error {
			if err := s.FS.Symlink("../other/b.txt", "in/l"); err != nil {
				return err
			}
			if target, err := s.FS.Readlink("in/l"); err != nil || target != "../other/b.txt" {
				t.Errorf("limit の Readlink = %q, %v", target, err)
			}
			got = s.FS.Symlink("a.txt", "other/l")
			if _, err := s.FS.Readlink("other/b.txt"); !errors.Is(err, outside.ErrOutsideRoot) {
				t.Errorf("宣言の根の外の Readlink = %v", err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if !errors.Is(got, outside.ErrOutsideRoot) {
			t.Errorf("宣言の根の外の Symlink = %v", got)
		}
	})
}
