package outside_test

import (
	"io/fs"
	"testing"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// TC-OUT-FS-PERM: NewFS の FS が作る file は 0644・dir は 0755、NewPrivateFS の FS が作る file は 0600・dir は 0700。
// Stat の Mode で読め、既に在る file への書き込みは 権限を変えない。real (perm_unix_test.go) と mock の両方に掛ける。

// permReporter は 契約の検査が誤りを知らせる先 (testing.T か 陽性対照の記録)。
type permReporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// wantMode は p の Stat の Mode が want かを確かめる。
func wantMode(r permReporter, f outside.FS, p string, want fs.FileMode) {
	r.Helper()
	info, err := f.Stat(p)
	if err != nil {
		r.Errorf("Stat(%s) = %v", p, err)
		return
	}
	if info.Mode != want {
		r.Errorf("Stat(%s).Mode = %#o (want %#o)", p, info.Mode, want)
	}
}

// permContract は 空の根から始まる f が 作る物に filePerm・dirPerm を当て 在る物の権限を変えない事を確かめる。
func permContract(r permReporter, f outside.FS, filePerm, dirPerm fs.FileMode) {
	r.Helper()
	ok := func(err error) bool {
		r.Helper()
		if err != nil {
			r.Errorf("%v", err)
			return false
		}
		return true
	}
	// WriteFile は 親の dir も作る。
	if !ok(f.WriteFile("w/x/a.txt", []byte("a"))) {
		return
	}
	wantMode(r, f, "w/x/a.txt", filePerm)
	wantMode(r, f, "w/x", dirPerm)
	wantMode(r, f, "w", dirPerm)
	// 在る file を書き直しても 権限は そのまま。
	if !ok(f.WriteFile("w/x/a.txt", []byte("aa"))) {
		return
	}
	wantMode(r, f, "w/x/a.txt", filePerm)

	// AppendFile: 無い file (親の dir も) と 在る file。
	if !ok(f.AppendFile("ap/y/log.txt", []byte("1"))) {
		return
	}
	wantMode(r, f, "ap/y/log.txt", filePerm)
	wantMode(r, f, "ap/y", dirPerm)
	if !ok(f.AppendFile("ap/y/log.txt", []byte("2"))) {
		return
	}
	wantMode(r, f, "ap/y/log.txt", filePerm)

	// MkdirAll の入れ子。在る dir の下に足しても 在る dir の権限は そのまま。
	if !ok(f.MkdirAll("m/n")) {
		return
	}
	wantMode(r, f, "m", dirPerm)
	wantMode(r, f, "m/n", dirPerm)
	if !ok(f.MkdirAll("m/n/o")) {
		return
	}
	wantMode(r, f, "m/n/o", dirPerm)

	// Rename は 権限を 持ったまま移す。
	if !ok(f.Rename("w/x/a.txt", "m/moved.txt")) {
		return
	}
	wantMode(r, f, "m/moved.txt", filePerm)

	// Lstat も 同じ Mode を返す。
	if info, err := f.Lstat("m/moved.txt"); err != nil || info.Mode != filePerm {
		r.Errorf("Lstat(m/moved.txt) = %+v, %v (want Mode %#o)", info, err, filePerm)
	}

	// 先の無い symlink へ書くと 先が 新しい file として作られる (権限は filePerm)。symlink の Mode は 0。
	if !ok(f.Symlink("m/target.txt", "dl")) || !ok(f.WriteFile("dl", []byte("d"))) {
		return
	}
	wantMode(r, f, "m/target.txt", filePerm)
	if info, err := f.Lstat("dl"); err != nil || info.Mode != 0 || !info.IsSymlink {
		r.Errorf("Lstat(dl) = %+v, %v (want symlink・Mode 0)", info, err)
	}
}

// permCrossContract は 同じ根を見る 既定の FS (def) と private の FS (priv) で
// 片方が作った物を もう片方が書いても 作った時の権限のまま である事を確かめる。根は 空から始まる。
func permCrossContract(r permReporter, def, priv outside.FS) {
	r.Helper()
	ok := func(err error) bool {
		r.Helper()
		if err != nil {
			r.Errorf("%v", err)
			return false
		}
		return true
	}
	if !ok(def.WriteFile("shared/s.txt", []byte("s"))) {
		return
	}
	if !ok(priv.WriteFile("shared/s.txt", []byte("t"))) || !ok(priv.AppendFile("shared/s.txt", []byte("u"))) {
		return
	}
	if !ok(priv.MkdirAll("shared")) {
		return
	}
	wantMode(r, priv, "shared/s.txt", 0o644)
	wantMode(r, priv, "shared", 0o755)
	// 既定の dir の中に private の FS が新しく作る file は 0600。
	if !ok(priv.WriteFile("shared/p.txt", []byte("p"))) {
		return
	}
	wantMode(r, def, "shared/p.txt", 0o600)
	// 逆向き: private の FS が作った物を 既定の FS で書いても 0600・0700 のまま。
	if !ok(priv.WriteFile("sec/k.txt", []byte("k"))) {
		return
	}
	if !ok(def.WriteFile("sec/k.txt", []byte("k2"))) || !ok(def.AppendFile("sec/k.txt", []byte("3"))) {
		return
	}
	wantMode(r, def, "sec/k.txt", 0o600)
	wantMode(r, def, "sec", 0o700)
}

func TestFSPerm_TC_OUT_FS_PERM_mock(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		f, err := outsidemock.NewFS(outsidemock.Tree{})
		if err != nil {
			t.Fatal(err)
		}
		permContract(t, f, 0o644, 0o755)
	})
	t.Run("private", func(t *testing.T) {
		f, err := outsidemock.NewPrivateFS(outsidemock.Tree{})
		if err != nil {
			t.Fatal(err)
		}
		permContract(t, f, 0o600, 0o700)
	})
	t.Run("cross", func(t *testing.T) {
		f, err := outsidemock.NewFS(outsidemock.Tree{})
		if err != nil {
			t.Fatal(err)
		}
		permCrossContract(t, f, f.PrivateView())
		// DefaultView も 同じ木を既定の権限で作る。
		p, err := outsidemock.NewPrivateFS(outsidemock.Tree{})
		if err != nil {
			t.Fatal(err)
		}
		permCrossContract(t, p.DefaultView(), p)
	})
	t.Run("tree", func(t *testing.T) {
		// tree で宣言した物の権限は その FS の既定。
		p, err := outsidemock.NewPrivateFS(outsidemock.Tree{"d/s.txt": "x", "e/": ""})
		if err != nil {
			t.Fatal(err)
		}
		wantMode(t, p, "d/s.txt", 0o600)
		wantMode(t, p, "d", 0o700)
		wantMode(t, p, "e", 0o700)
		f, err := outsidemock.NewFS(outsidemock.Tree{"d/s.txt": "x"})
		if err != nil {
			t.Fatal(err)
		}
		wantMode(t, f, "d/s.txt", 0o644)
		wantMode(t, f, "d", 0o755)
	})
}
