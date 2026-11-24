//go:build !unix

package outside

import (
	"errors"
	"os/exec"
	"time"
)

// この file も 本物の実装 (real)。process group を持たない OS では Cmd.KillGroup を 誤りにする (黙って子だけを止めない)。

type killGroup struct{}

func newKillGroup(*exec.Cmd, time.Duration, *procStop) (*killGroup, error) {
	return nil, errors.New("KillGroup は process group を持たない OS では使えない")
}

func (*killGroup) finish() {}
