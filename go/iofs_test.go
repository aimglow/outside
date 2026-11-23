package outside_test

import (
	"testing"
	"testing/fstest"

	"github.com/aimglow/outside/go"
	"github.com/aimglow/outside/go/outsidemock"
)

// TC-OUT-IOFS: FS を io/fs の決まり (fstest.TestFS) どおりに読ませる。
func TestIOFS_TC_OUT_IOFS(t *testing.T) {
	o, err := outsidemock.NewFS(outsidemock.Tree{"a.txt": "a\n", "d/b.txt": "bb\n", "d/e/c.txt": "c"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fstest.TestFS(outside.IOFS(o), "a.txt", "d/b.txt", "d/e/c.txt"); err != nil {
		t.Fatal(err)
	}
}
