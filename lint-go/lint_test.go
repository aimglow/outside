// TC-OUT-LINT-GO: 利用する project が取り込む golangci.yml が 外を直に呼ぶ code を落とし 口を通す code を通す。
// 実の golangci-lint を この repo の fixture (testdata) にだけ掛ける (この repo の中で 実物を確かめる test)。
package lintgo

import (
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"testing"
)

// issueRe は golangci-lint の 1 件の行 (bad.go の行の番号と 名指した呼び出し)。
var issueRe = regexp.MustCompile("bad/bad\\.go:(\\d+):\\d+: use of `([^`]+)` forbidden")

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
	// 期待 = bad.go の行 → 名指す呼び出し (import の別名のまま名指される)。種類ごとに 1 つ以上。
	want := map[int]string{
		14: "time.Now",
		15: "os.Getenv",
		16: "exec.Command",
		17: "os.MkdirTemp",
		18: "os.ReadFile",
		19: "os.Hostname",
		20: "os.Getpid",
		21: "os.Getppid",
		22: "os.Getwd",
		23: "os.UserHomeDir",
		24: "os.UserConfigDir",
		25: "os.UserCacheDir",
		26: "rand.Intn",
		27: "randv2.IntN",
		28: "crand.Read",
		29: "crand.Text",
		30: "crand.Prime",
		31: "crand.Reader",
		32: "os.Symlink",
		33: "os.Readlink",
		34: "os.Link",
	}
	got := map[int]string{}
	for _, m := range issueRe.FindAllStringSubmatch(out, -1) {
		line, _ := strconv.Atoi(m[1])
		got[line] = m[2]
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("名指した行 = %v\n期待 = %v\n出力:\n%s", got, want, out)
	}
}

func TestLintGo_TC_OUT_LINT_GO_通す(t *testing.T) {
	out, code := run(t, "./good/...")
	if code != 0 {
		t.Fatalf("口を通す code が落ちた (code=%d):\n%s", code, out)
	}
}
