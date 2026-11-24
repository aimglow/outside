package bad

import (
	crand "crypto/rand"
	"math/rand"
	randv2 "math/rand/v2"
	"os"
	"os/exec"
	"time"
)

// Bad は 外を直に呼ぶ (lint が落とすべき形)。1 行に 1 つ (lint は 同じ行の 2 つ目を名指さない)。
func Bad() {
	_ = time.Now()
	_ = os.Getenv("A")
	_ = exec.Command("x")
	_, _ = os.MkdirTemp("", "x")
	_, _ = os.ReadFile("a")
	_, _ = os.Hostname()
	_ = os.Getpid()
	_ = os.Getppid()
	_, _ = os.Getwd()
	_, _ = os.UserHomeDir()
	_, _ = os.UserConfigDir()
	_, _ = os.UserCacheDir()
	_ = rand.Intn(3)
	_ = randv2.IntN(3)
	_, _ = crand.Read(nil)
	_ = crand.Text()
	_, _ = crand.Prime(nil, 8)
	_ = crand.Reader
}
