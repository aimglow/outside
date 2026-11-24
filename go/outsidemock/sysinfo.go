package outsidemock

import "fmt"

// SysInfoValues は mock の SysInfo が返す値。test が値として宣言する。
type SysInfoValues struct {
	// Hostname が空なら Hostname は 誤り。
	Hostname string
	// HomeDir が空なら HomeDir は 誤り。
	HomeDir string
	Pid     int
	IsTTY   bool
}

// SysInfo は 渡した値を返す mock。機体と process の値を読まない。
type SysInfo struct {
	recorder
	v SysInfoValues
}

// NewSysInfo は v を返す SysInfo を作る。
func NewSysInfo(v SysInfoValues) *SysInfo { return &SysInfo{v: v} }

func (s *SysInfo) Hostname() (string, error) {
	s.record("Hostname")
	if s.v.Hostname == "" {
		return "", fmt.Errorf("outsidemock: host の名前が宣言されていない")
	}
	return s.v.Hostname, nil
}

func (s *SysInfo) HomeDir() (string, error) {
	s.record("HomeDir")
	if s.v.HomeDir == "" {
		return "", fmt.Errorf("outsidemock: home の dir が宣言されていない")
	}
	return s.v.HomeDir, nil
}

func (s *SysInfo) Pid() int {
	s.record("Pid")
	return s.v.Pid
}

func (s *SysInfo) IsTTY() bool {
	s.record("IsTTY")
	return s.v.IsTTY
}
