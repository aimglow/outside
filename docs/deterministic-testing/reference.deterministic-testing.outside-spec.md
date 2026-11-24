# 04. `libs/outside/{go,angular,next,flutter}` 標準仕様と実装リファレンス

本章では、Nx ワークスペースの `libs/outside/` 配下に配置する 4 つのランタイム基盤の標準インターフェースと、`Real`（本番用）・`Mock`（テスト用）の完全な実装リファレンスを定義します。

---

## 1. 全ランタイム共通の設計原則

1. **ペア提供の原則:** すべてのプリミティブは `Real` 実装と `Mock` 実装のセットで提供する。
2. **ゼロ汚染の原則:** `Mock` を生成（`NewMock` / `createMockOutside` / `Outside.mock`）するたびに完全に独立したインメモリ状態が作られ、テスト間の状態汚染が起きないこと。
3. **検証ヘルパーの同梱:** `Mock` 側には、テストのアサーションを簡潔にするためのヘルパー（仮想環境変数のセット、コマンド実行履歴 `execHistory`、仮想時計の進行 `Advance` など）を標準装備する。

---

## 2. `libs/outside/go`（Go ランタイム）

> **実物の名前（正）:** 口の組は `Set`（`Clock`・`Env`・`FS`・`Proc`・`Workdir`・`SysInfo`・`IdGen`）、mock は package `outsidemock`、入口は `RunTool`（ツールの dir の `outside.toml` で 使う環境を宣言する）。下のコードは 考え方の参照で、名前は実物に従う。特定ツール名を持つ口（`Git` 等）は `outside` に置かず adapters 層（`libs/platform/vcs-git-go` 等）に置く。依存に許すのは 標準ライブラリと `github.com/BurntSushi/toml`（`outside.toml` の読み取り）だけ。

AI開発基盤（Session Relay・台帳・監視 Daemon）を完全に決定論化するため、`afero.Fs` に加えて **タイマー進行（`Advance`）付き仮想時計**・**決定論的 `IdGen`**・**固定 `SysInfo`** を備えます。

```go
// libs/outside/go/outside.go
package outside

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/afero"
)

// Clock は時刻取得およびタイマー待機を抽象化する。
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

// CmdRunner は外部プロセス実行を抽象化する。
type CmdRunner interface {
	Run(name string, args ...string) ([]byte, error)
	// RunWithStdin は stdin を子プロセスの標準入力に渡す（秘密を argv にもファイルにも載せない）。
	RunWithStdin(stdin []byte, name string, args ...string) ([]byte, error)
}

// SysInfo は実行マシン・プロセス固有情報を抽象化する。
type SysInfo interface {
	Hostname() string
	HomeDir() string
	Pid() int
	IsTTY() bool
}

// IdGen は決定論的な ID・乱数生成を抽象化する。
type IdGen interface {
	NewID() string
}

// Env は Go アプリケーションが必要とする全 OS 境界を束ねる。
type Env struct {
	Fs      afero.Fs
	Clock   Clock
	Cmd     CmdRunner
	Sys     SysInfo
	Id      IdGen
	Getenv  func(key string) string
	// Mock 検証用フィールド（Real では nil）
	Mock *MockControls
}

// MockControls はテスト時のみ使用する操作・検証用ハンドル。
type MockControls struct {
	Clock       *MockClock
	Cmd         *MockCmdRunner
	EnvVars     map[string]string
	mu          sync.Mutex
}

func (f *MockControls) SetEnv(key, val string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.EnvVars[key] = val
}

// --- Real 実装 ---

type realClock struct{}
func (realClock) Now() time.Time        { return time.Now().UTC() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

type realCmdRunner struct{}
func (realCmdRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}
func (realCmdRunner) RunWithStdin(stdin []byte, name string, args ...string) ([]byte, error) {
	c := exec.Command(name, args...)
	c.Stdin = bytes.NewReader(stdin)
	return c.CombinedOutput()
}

type realSysInfo struct{}
func (realSysInfo) Hostname() string { h, _ := os.Hostname(); return h }
func (realSysInfo) HomeDir() string  { d, _ := os.UserHomeDir(); return d }
func (realSysInfo) Pid() int         { return os.Getpid() }
func (realSysInfo) IsTTY() bool      { return false }

type realIdGen struct{ counter atomic.Uint64 }
func (r *realIdGen) NewID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), r.counter.Add(1))
}

// NewReal は本番用の OS 境界セットを生成する。
func NewReal() *Env {
	return &Env{
		Fs:     afero.NewOsFs(),
		Clock:  realClock{},
		Cmd:    realCmdRunner{},
		Sys:    realSysInfo{},
		Id:     &realIdGen{},
		Getenv: os.Getenv,
	}
}

// --- Mock 実装 ---

type MockClock struct {
	mu      sync.Mutex
	current time.Time
}

func (c *MockClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *MockClock) Sleep(d time.Duration) {
	c.Advance(d)
}

func (c *MockClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = c.current.Add(d)
}

type RecordedCmd struct {
	Name  string
	Args  []string
	Stdin []byte
}

type MockCmdRunner struct {
	mu      sync.Mutex
	History []RecordedCmd
	StubOut []byte
	StubErr error
}

func (c *MockCmdRunner) Run(name string, args ...string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.History = append(c.History, RecordedCmd{Name: name, Args: args})
	return c.StubOut, c.StubErr
}

func (c *MockCmdRunner) RunWithStdin(stdin []byte, name string, args ...string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.History = append(c.History, RecordedCmd{Name: name, Args: args, Stdin: append([]byte(nil), stdin...)})
	return c.StubOut, c.StubErr
}

type mockSysInfo struct{}
func (mockSysInfo) Hostname() string { return "test-host" }
func (mockSysInfo) HomeDir() string  { return "/virtual/home" }
func (mockSysInfo) Pid() int         { return 1000 }
func (mockSysInfo) IsTTY() bool      { return false }

type mockIdGen struct{ seq atomic.Uint64 }
func (g *mockIdGen) NewID() string {
	return fmt.Sprintf("id-%04d", g.seq.Add(1))
}

// NewMock はテスト用の 100% 決定論的なインメモリ環境を生成する。
func NewMock(fixedTime time.Time) *Env {
	fc := &MockClock{current: fixedTime.UTC()}
	fcmd := &MockCmdRunner{}
	envVars := make(map[string]string)
	controls := &MockControls{
		Clock:   fc,
		Cmd:     fcmd,
		EnvVars: envVars,
	}
	return &Env{
		Fs:    afero.NewMemMapFs(),
		Clock: fc,
		Cmd:   fcmd,
		Sys:   mockSysInfo{},
		Id:    &mockIdGen{},
		Getenv: func(key string) string {
			controls.mu.Lock()
			defer controls.mu.Unlock()
			return envVars[key]
		},
		Mock: controls,
	}
}
```

---

## 3. `libs/outside/next`（Next.js / Node.js ランタイム）

> **実物の名前（正）:** 口の組は `clock`・`http`・`storage`・`config`・`idGen`（`createRealSet` / `createMockSet`、mock は `@aimglow/outside-next/mock`）。`dependencies` は空で `memfs` は使わない。下のコードは 考え方の参照で、名前と口の組は実物と `design.outside.yaml` の `ports` に従う。

サーバーサイド（Route Handler / Server Action / UseCase）向けの OS 境界です。

```typescript
// libs/outside/next/index.ts
import * as realFs from 'node:fs/promises';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { randomUUID } from 'node:crypto';
import { Volume, createFsFromVolume } from 'memfs';

const execFileAsync = promisify(execFile);

export interface Outside {
  fs: typeof realFs;
  now: () => Date;
  env: (key: string) => string | undefined;
  exec: (cmd: string, args: string[]) => Promise<string>;
  newId: () => string;
}

export interface MockOutside extends Outside {
  setEnv: (key: string, value: string) => void;
  advanceTimeMs: (ms: number) => void;
  stubExec: (stdout: string) => void;
  execHistory: Array<{ cmd: string; args: string[] }>;
}

export function createRealOutside(): Outside {
  return {
    fs: realFs,
    now: () => new Date(),
    env: (key) => process.env[key],
    exec: async (cmd, args) => {
      const { stdout } = await execFileAsync(cmd, args, { encoding: 'utf-8' });
      return stdout;
    },
    newId: () => randomUUID(),
  };
}

export function createMockOutside(fixedIso = '2026-01-01T00:00:00.000Z'): MockOutside {
  const vol = new Volume();
  let currentMs = Date.parse(fixedIso);
  const envMap = new Map<string, string>();
  let nextStdout = '';
  let idSeq = 0;
  const execHistory: Array<{ cmd: string; args: string[] }> = [];

  return {
    fs: createFsFromVolume(vol).promises as unknown as typeof realFs,
    now: () => new Date(currentMs),
    env: (key) => envMap.get(key),
    exec: async (cmd, args) => {
      execHistory.push({ cmd, args });
      return nextStdout;
    },
    newId: () => {
      idSeq += 1;
      return `id-${String(idSeq).padStart(4, '0')}`;
    },
    setEnv: (key, value) => {
      envMap.set(key, value);
    },
    advanceTimeMs: (ms) => {
      currentMs += ms;
    },
    stubExec: (stdout) => {
      nextStdout = stdout;
    },
    execHistory,
  };
}
```

---

## 4. `libs/outside/angular`（Angular / ブラウザ ランタイム）

> **実物の名前（正）:** 口の組は `clock`・`http`・`storage`・`config`・`idGen`（`createRealSet` / `createMockSet`、mock は `@aimglow/outside-angular/mock`）。下のコードは 考え方の参照で、名前と口の組は実物と `design.outside.yaml` の `ports` に従う。

ブラウザ環境には Node.js の `fs` や `process.env` が存在しないため、ブラウザ固有の非決定要素（時計・乱数・Web Storage・Location 情報）を隔離します。

```typescript
// libs/outside/angular/index.ts
export interface KeyValueStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
  clear(): void;
}

export interface BrowserOutside {
  now: () => Date;
  newId: () => string;
  localStorage: KeyValueStorage;
  sessionStorage: KeyValueStorage;
  origin: () => string;
}

export interface MockBrowserOutside extends BrowserOutside {
  advanceTimeMs: (ms: number) => void;
  setOrigin: (origin: string) => void;
}

function createMemoryStorage(): KeyValueStorage {
  const store = new Map<string, string>();
  return {
    getItem: (k) => store.get(k) ?? null,
    setItem: (k, v) => { store.set(k, v); },
    removeItem: (k) => { store.delete(k); },
    clear: () => { store.clear(); },
  };
}

export function createRealBrowserOutside(): BrowserOutside {
  return {
    now: () => new Date(),
    newId: () => crypto.randomUUID(),
    localStorage: window.localStorage,
    sessionStorage: window.sessionStorage,
    origin: () => window.location.origin,
  };
}

export function createMockBrowserOutside(
  fixedIso = '2026-01-01T00:00:00.000Z',
): MockBrowserOutside {
  let currentMs = Date.parse(fixedIso);
  let idSeq = 0;
  let currentOrigin = 'https://app.example.test';

  return {
    now: () => new Date(currentMs),
    newId: () => {
      idSeq += 1;
      return `id-${String(idSeq).padStart(4, '0')}`;
    },
    localStorage: createMemoryStorage(),
    sessionStorage: createMemoryStorage(),
    origin: () => currentOrigin,
    advanceTimeMs: (ms) => { currentMs += ms; },
    setOrigin: (origin) => { currentOrigin = origin; },
  };
}
```

---

## 5. `libs/outside/flutter`（Dart / Flutter ランタイム）

> **実物の名前（正）:** 口の組は `Clock`・`Http`・`Storage`・`Config`・`IdGen`（mock は `package:outside/mock.dart`）。`dependencies` は空（`package:file`・`package:clock`・`package:http` も使わない。Http の real は `dart:io` の `HttpClient`）。下のコードは 考え方の参照で、名前と口の組は実物と `design.outside.yaml` の `ports` に従う。

`package:flutter` を一切 `import` しない Pure Dart 構成とすることで、UI なしの高速な `dart test` / `flutter test` キャッシュを保証します。

```dart
// libs/outside/flutter/lib/outside.dart
import 'dart:io' as io;
import 'package:file/file.dart';
import 'package:file/local.dart';
import 'package:file/memory.dart';
import 'package:clock/clock.dart';

abstract class ProcessRunner {
  Future<String> run(String executable, List<String> arguments);
}

class RealProcessRunner implements ProcessRunner {
  @override
  Future<String> run(String executable, List<String> arguments) async {
    final result = await io.Process.run(executable, arguments);
    if (result.exitCode != 0) {
      throw StateError('Process failed (${result.exitCode}): ${result.stderr}');
    }
    return result.stdout.toString();
  }
}

class MockProcessRunner implements ProcessRunner {
  String stubStdout = '';
  final List<String> history = [];

  @override
  Future<String> run(String executable, List<String> arguments) async {
    history.add('$executable ${arguments.join(' ')}'.trim());
    return stubStdout;
  }
}

class Outside {
  final FileSystem fs;
  final Clock clock;
  final ProcessRunner process;
  final String? Function(String key) env;
  final String Function() newId;

  // Mock 操作用ハンドル（Real では null）
  final MockProcessRunner? mockProcess;
  final Map<String, String>? mockEnvMap;

  Outside({
    required this.fs,
    required this.clock,
    required this.process,
    required this.env,
    required this.newId,
    this.mockProcess,
    this.mockEnvMap,
  });

  factory Outside.real() {
    var seq = 0;
    return Outside(
      fs: const LocalFileSystem(),
      clock: const Clock(),
      process: RealProcessRunner(),
      env: (key) => io.Platform.environment[key],
      newId: () {
        seq += 1;
        return '${DateTime.now().toUtc().microsecondsSinceEpoch}-$seq';
      },
    );
  }

  factory Outside.mock({DateTime? fixedTime}) {
    final baseTime = (fixedTime ?? DateTime.utc(2026, 1, 1)).toUtc();
    final mockProc = MockProcessRunner();
    final envMap = <String, String>{};
    var seq = 0;

    return Outside(
      fs: MemoryFileSystem(),
      clock: Clock.fixed(baseTime),
      process: mockProc,
      env: (key) => envMap[key],
      newId: () {
        seq += 1;
        return 'id-${seq.toString().padLeft(4, '0')}';
      },
      mockProcess: mockProc,
      mockEnvMap: envMap,
    );
  }
}
```
