package outsidemock

import "fmt"

// SysInfoValues は mock の SysInfo が返す値。test が値として宣言する。
type SysInfoValues struct {
	// Hostname が空なら Hostname は 誤り。
	Hostname string
	// HomeDir が空なら HomeDir は 誤り。
	HomeDir string
	// ConfigDir・CacheDir・Getwd が空なら それぞれ 誤り。
	ConfigDir string
	CacheDir  string
	Getwd     string
	Pid       int
	Ppid      int
	IsTTY     bool
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

func (s *SysInfo) ConfigDir() (string, error) {
	s.record("ConfigDir")
	if s.v.ConfigDir == "" {
		return "", fmt.Errorf("outsidemock: user の設定の置き場が宣言されていない")
	}
	return s.v.ConfigDir, nil
}

func (s *SysInfo) CacheDir() (string, error) {
	s.record("CacheDir")
	if s.v.CacheDir == "" {
		return "", fmt.Errorf("outsidemock: user の cache の置き場が宣言されていない")
	}
	return s.v.CacheDir, nil
}

func (s *SysInfo) Getwd() (string, error) {
	s.record("Getwd")
	if s.v.Getwd == "" {
		return "", fmt.Errorf("outsidemock: 作業 dir が宣言されていない")
	}
	return s.v.Getwd, nil
}

func (s *SysInfo) Ppid() int {
	s.record("Ppid")
	return s.v.Ppid
}

func (s *SysInfo) Pid() int {
	s.record("Pid")
	return s.v.Pid
}

func (s *SysInfo) IsTTY() bool {
	s.record("IsTTY")
	return s.v.IsTTY
}
