// TC-OUT-LINT-GO: 利用する project が取り込む golangci.yml が 外を直に呼ぶ code を落とし 口を通す code を通す。
// 実の golangci-lint を この repo の fixture (testdata) にだけ掛ける (この repo の中で 実物を確かめる test)。
package lintgo

import (
	"os/exec"
	"strings"
	"testing"
)

// run は fixture の dir で golangci-lint を撃ち 出力と終了の番号を返す。
func run(t *testing.T, target string) (string, int) {
	t.Helper()
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Fatalf("golangci-lint が無い (この test の使う環境 = golangci-lint 2.x): %v", err)
	}
	cmd := exec.Command("golangci-lint", "run", "--config", "../golangci.yml", target)
	cmd.Dir = "testdata"
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("golangci-lint を起こせない: %v", err)
	}
	return string(out), code
}

func TestLintGo_TC_OUT_LINT_GO_落とす(t *testing.T) {
	out, code := run(t, "./bad/...")
	if code == 0 {
		t.Fatalf("外を直に呼ぶ code が通った:\n%s", out)
	}
	for _, want := range []string{"time.Now", "os.Getenv", "exec.Command", "os.MkdirTemp", "os.ReadFile"} {
		if !strings.Contains(out, "`"+want+"`") {
			t.Errorf("%s を名指していない:\n%s", want, out)
		}
	}
}

func TestLintGo_TC_OUT_LINT_GO_通す(t *testing.T) {
	out, code := run(t, "./good/...")
	if code != 0 {
		t.Fatalf("口を通す code が落ちた (code=%d):\n%s", code, out)
	}
}
