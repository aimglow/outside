package bad

import (
	"os"
	"os/exec"
	"time"
)

// Bad は 外を直に呼ぶ (lint が落とすべき形)。
func Bad() {
	_ = time.Now()
	_ = os.Getenv("A")
	_ = exec.Command("git")
	_, _ = os.MkdirTemp("", "x")
	_, _ = os.ReadFile("a")
}
