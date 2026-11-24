// Package outside は application とツールが 外を触る口 (port) を定める。
// 口の定義の正本は docs/developer/design.outside.yaml の ports の節。
package outside

import (
	"context"
	"errors"
	"time"

	"github.com/aimglow/outside/go/internal/rule"
)

var (
	// ErrOutsideRoot は 宣言した根の外の path を指した時の誤り。
	ErrOutsideRoot = rule.ErrOutsideRoot
	// ErrUndeclaredProgram は 宣言に無い program を起こそうとした時の誤り。
	ErrUndeclaredProgram = errors.New("outside: 宣言に無い program")
	// ErrNoWorkdirRoot は 作業場の置き場が 宣言されていないか 在らない時の誤り。
	ErrNoWorkdirRoot = errors.New("outside: 作業場の置き場が無い")
	// ErrUndetermined は 判定不能 (宣言が読めない・環境が宣言に合わない) を表す。ツールの合否とは別。
	ErrUndetermined = errors.New("outside: 判定不能")
)

// Clock は 今の時刻を返し 待つ。
type Clock interface {
	Now() time.Time
	// Sleep は d だけ待つ。0 以下は 待たない。
	Sleep(d time.Duration)
}

// Env は 環境変数を名前で読む。無い名前は ok=false。
type Env interface {
	Lookup(name string) (value string, ok bool)
}

// DirEntry は ReadDir の 1 件。
type DirEntry struct {
	Name  string
	IsDir bool
}

// FileInfo は Stat の結果。dir の Size は 0。
type FileInfo struct {
	Name  string
	Size  int64
	IsDir bool
}

// FS は 宣言した根の下の file を読む・書く・一覧する。
// path は 根からの相対 (区切りは /)。根の外の path は ErrOutsideRoot。
// 無い path は errors.Is(err, fs.ErrNotExist) で判る。
type FS interface {
	ReadFile(path string) ([]byte, error)
	// WriteFile は 親の dir が無ければ作って書く。
	WriteFile(path string, data []byte) error
	// ReadDir は 名前の順で返す。
	ReadDir(path string) ([]DirEntry, error)
	Stat(path string) (FileInfo, error)
	// Remove は file か 空の dir を消す。
	Remove(path string) error
}

// SysInfo は 動いている機体と process の値を返す。
type SysInfo interface {
	// Hostname は host の名前を返す。引けなければ 誤り (空の文字で黙らない)。
	Hostname() (string, error)
	// HomeDir は 動かしている user の home の dir を返す。引けなければ 誤り。
	HomeDir() (string, error)
	// Pid は この process の番号を返す。
	Pid() int
	// IsTTY は 標準入力が 端末 (文字 device) かを返す。
	IsTTY() bool
}

// IdGen は 新しい ID を返す。
type IdGen interface {
	// NewID は 呼ぶたびに 違う ID を返す。
	NewID() string
}

// Cmd は 起こす program と その引数。
type Cmd struct {
	Name string
	Args []string
	// Dir は 子 process の作業 dir。空なら 親と同じ。
	Dir string
	// Env は 子 process に足す環境変数 (NAME=値)。
	Env []string
	// Stdin は 子 process の標準入力に渡す中身。nil なら 何も渡さない。
	// 秘密 (鍵・password) を disk にも argv にも載せずに渡す時に使う。
	Stdin []byte
}

// Result は 子 process の結果。0 以外の ExitCode も 誤りではなく Result で返る。
type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// Proc は 宣言した名前の program を起こし 結果を返す。宣言に無い名前は ErrUndeclaredProgram。
type Proc interface {
	Run(ctx context.Context, cmd Cmd) (Result, error)
}

// Workdir は 宣言した置き場の下に 作業場を作り 終わりに消す。
type Workdir interface {
	// New は 置き場の直下に name の作業場を作り path を返す。同じ名前が在れば 誤り。
	New(name string) (string, error)
	// Path は 最後に作った作業場の path を返す。無ければ 空。
	Path() string
	// Cleanup は この口が作った作業場を 全て消す。
	Cleanup() error
	// FS は この口が作った作業場 path を根にした FS を返す。この口が作っていない path は 誤り。
	FS(path string) (FS, error)
}

// Set は 1 つの処理に渡す port の組。
type Set struct {
	Clock   Clock
	Env     Env
	FS      FS
	Proc    Proc
	Workdir Workdir
	SysInfo SysInfo
	IdGen   IdGen
}
