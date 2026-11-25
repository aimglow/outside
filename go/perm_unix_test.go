//go:build unix

package outside_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/aimglow/outside/go"
)

// TC-OUT-FS-PERM の real 側。権限の bit を持つ unix で回す。
// umask を 077 と 022 にして 同じ Mode が出る事 (umask に依らない) を確かめる。
// umask は process 全体の値ゆえ この file の test は t.Parallel を使わず 終わりに必ず戻す。

// withUmask は umask を mask にして fn を呼び 前の値へ戻す。
func withUmask(t *testing.T, mask int, fn func()) {
	t.Helper()
	old := syscall.Umask(mask)
	defer syscall.Umask(old)
	fn()
}

// subDir は test の dir の下に name の dir を作る (根にする dir。権限は umask に依らず 0700)。
func subDir(t *testing.T, base, name string) string {
	t.Helper()
	p := filepath.Join(base, name)
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFSPerm_TC_OUT_FS_PERM_real(t *testing.T) {
	for _, mask := range []int{0o077, 0o022} {
		t.Run(fmt.Sprintf("umask%03o", mask), func(t *testing.T) {
			base := testDir(t)
			withUmask(t, mask, func() {
				def, err := outside.NewFS(subDir(t, base, "def"))
				if err != nil {
					t.Fatal(err)
				}
				permContract(t, def, 0o644, 0o755)

				priv, err := outside.NewPrivateFS(subDir(t, base, "priv"))
				if err != nil {
					t.Fatal(err)
				}
				permContract(t, priv, 0o600, 0o700)

				shared := subDir(t, base, "shared")
				d2, err := outside.NewFS(shared)
				if err != nil {
					t.Fatal(err)
				}
				p2, err := outside.NewPrivateFS(shared)
				if err != nil {
					t.Fatal(err)
				}
				permCrossContract(t, d2, p2)
			})
		})
	}
}

// NewPrivateFS は 根の権限を変えない。
func TestFSPerm_TC_OUT_FS_PERM_real_根は変えない(t *testing.T) {
	root := subDir(t, testDir(t), "root")
	if err := os.Chmod(root, 0o750); err != nil {
		t.Fatal(err)
	}
	f, err := outside.NewPrivateFS(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFile("a.txt", []byte("a")); err != nil {
		t.Fatal(err)
	}
	wantMode(t, f, ".", 0o750)
}

// permLog は 陽性対照で 契約の検査が出した誤りを数える。
type permLog struct{ msgs []string }

func (l *permLog) Helper() {}
func (l *permLog) Errorf(format string, args ...any) {
	l.msgs = append(l.msgs, fmt.Sprintf(format, args...))
}

// noFixFS は realFS の 作る所から Chmod を外した写し (作る時に権限を渡すだけ = umask に削られる)。
// 読む所は 本物の FS に任せる。陽性対照だけが使う。
type noFixFS struct {
	outside.FS
	root              string
	filePerm, dirPerm fs.FileMode
}

func (f noFixFS) WriteFile(p string, data []byte) error {
	abs := filepath.Join(f.root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(abs), f.dirPerm); err != nil {
		return err
	}
	return os.WriteFile(abs, data, f.filePerm)
}

func (f noFixFS) AppendFile(p string, data []byte) error {
	abs := filepath.Join(f.root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(abs), f.dirPerm); err != nil {
		return err
	}
	file, err := os.OpenFile(abs, os.O_WRONLY|os.O_APPEND|os.O_CREATE, f.filePerm)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (f noFixFS) MkdirAll(p string) error {
	return os.MkdirAll(filepath.Join(f.root, filepath.FromSlash(p)), f.dirPerm)
}

// 陽性対照: Chmod を外した写しは umask 077 の時に 既定の FS の file が 0600・dir が 0700 になり 検査が赤くなる。
func TestFSPerm_TC_OUT_FS_PERM_陽性対照(t *testing.T) {
	base := testDir(t)
	withUmask(t, 0o077, func() {
		root := subDir(t, base, "nofix")
		inner, err := outside.NewFS(root)
		if err != nil {
			t.Fatal(err)
		}
		log := &permLog{}
		permContract(log, noFixFS{FS: inner, root: root, filePerm: 0o644, dirPerm: 0o755}, 0o644, 0o755)
		if len(log.msgs) == 0 {
			t.Fatal("Chmod を外した写しで 検査が赤くならない (検査が成り立たない)")
		}
		want := "Stat(w/x/a.txt).Mode = 0600 (want 0644)"
		if log.msgs[0] != want {
			t.Errorf("陽性対照の最初の誤り = %q (want %q)", log.msgs[0], want)
		}
	})
}
