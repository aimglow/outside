# 06. 機械的強制ルール（Lint・Nx モジュール境界・キャッシュ最適化）

AI エージェントに規約を守らせる最大の鍵は、プロンプトでの説得ではなく **「違反コードを書いた瞬間に Lint / ビルドが即座に赤くなり、自己修正せざるを得ない状態を作ること」** です。

---

## 1. Nx モジュール境界の強制（`@nx/enforce-module-boundaries`）

各プロジェクトの `project.json` に 層の tag（名前空間 `layer:`）を付与します。project が既に使っている `scope:` 等の軸とは別の名前空間にし 混ぜない。

- `libs/outside/*`: `["layer:outside"]`
- `libs/platform/*`: `["layer:adapter"]`
- `apps/*`: `["layer:app"]`

ワークスペースルートの `eslint.config.js` で依存方向をロックします。

```javascript
// eslint.config.js（Nx モジュール境界設定抜粋）
{
  files: ['**/*.ts', '**/*.tsx', '**/*.js'],
  rules: {
    '@nx/enforce-module-boundaries': [
      'error',
      {
        enforceBuildableLibDependency: true,
        allow: [],
        depConstraints: [
          {
            // outside は他への依存を一切持たない（純粋なリーフ基盤）
            sourceTag: 'layer:outside',
            onlyDependOnLibsWithTags: ['layer:outside'],
          },
          {
            // adapter は outside と他の adapter にのみ依存可能
            sourceTag: 'layer:adapter',
            onlyDependOnLibsWithTags: ['layer:outside', 'layer:adapter'],
          },
          {
            // app は adapter と outside に依存可能
            sourceTag: 'layer:app',
            onlyDependOnLibsWithTags: ['layer:outside', 'layer:adapter', 'layer:app'],
          },
        ],
      },
    ],
  },
}
```

---

## 2. TypeScript（Next.js / Angular）禁止構文・実機パス禁止 ESLint

`libs/outside/**` および `**/adapters/**/real.ts` **以外**のすべてのファイル（ビジネスロジック、`mock.ts`、Unit テスト）に適用します。

```javascript
// eslint.config.js（非決定要素・生IO・実機絶対パス禁止ルール）
{
  files: ['**/*.ts', '**/*.tsx'],
  ignores: [
    'libs/outside/**',
    '**/adapters/**/real.ts',
    '**/main.ts',
  ],
  rules: {
    // 1. 生の Node.js IO・プロセスの直接 import 禁止
    'no-restricted-imports': [
      'error',
      {
        paths: [
          { name: 'fs', message: 'out.fs を使用してください' },
          { name: 'node:fs', message: 'out.fs を使用してください' },
          { name: 'node:fs/promises', message: 'out.fs を使用してください' },
          { name: 'child_process', message: 'out.exec を使用してください' },
          { name: 'node:child_process', message: 'out.exec を使用してください' },
          { name: 'os', message: '環境依存になるため禁止です' },
          { name: 'node:os', message: '環境依存になるため禁止です' },
          { name: 'next/headers', message: 'adapters/auth-cookie 等を経由してください' },
        ],
      },
    ],
    // 2. 実行ディレクトリ・実環境変数の参照禁止
    'no-restricted-properties': [
      'error',
      { object: 'process', property: 'cwd', message: '実行パス依存になるため禁止です' },
      { object: 'process', property: 'env', message: 'out.env() を使用してください' },
      { object: 'Date', property: 'now', message: 'out.now() を使用してください' },
      { object: 'Math', property: 'random', message: 'out.newId() を使用してください' },
      { object: 'crypto', property: 'randomUUID', message: 'out.newId() を使用してください' },
      { object: 'window', property: 'localStorage', message: 'out.localStorage を使用してください' },
      { object: 'window', property: 'sessionStorage', message: 'out.sessionStorage を使用してください' },
    ],
    'no-restricted-globals': [
      'error',
      { name: '__dirname', message: '実行パス依存になるため禁止です' },
      { name: '__filename', message: '実行パス依存になるため禁止です' },
      { name: 'localStorage', message: 'out.localStorage を使用してください' },
      { name: 'sessionStorage', message: 'out.sessionStorage を使用してください' },
    ],
    // 3. 引数なし new Date() および実機固有の絶対パスハードコード禁止
    'no-restricted-syntax': [
      'error',
      {
        selector: "NewExpression[callee.name='Date'][arguments.length=0]",
        message: '引数なしの new Date() は禁止です。out.now() を使用してください',
      },
      {
        selector: "Literal[value=/^(\/Users\/|\/home\/|\/var\/|\/private\/|\/tmp\/|[A-Za-z]:\\)/]",
        message: '実機パスは禁止です。仮想パス（例: "/virtual/..."）を使用してください',
      },
    ],
  },
}
```

---

## 3. Go 禁止構文・実機パス禁止 `.golangci.yml`

```yaml
# .golangci.yml
linters:
  enable:
    - forbidigo

linters-settings:
  forbidigo:
    forbid:
      - p: '^time\.Now$'
        msg: "直接 time.Now() を呼ばず、env.Clock.Now() を使用してください"
      - p: '^time\.Sleep$'
        msg: "直接 time.Sleep() を呼ばず、env.Clock.Sleep() を使用してください"
      - p: '^os\.(Open|Create|ReadFile|WriteFile|MkdirAll|Remove|RemoveAll|Stat)$'
        msg: "直接 os のファイル操作を行わず、env.Fs (afero.Fs) を使用してください"
      - p: '^os\.(Getenv|LookupEnv|Setenv)$'
        msg: "直接環境変数を参照せず、env.Getenv() を使用してください"
      - p: '^os\.(Getwd|TempDir|UserHomeDir|UserConfigDir|UserCacheDir|Hostname|Getpid)$'
        msg: "実行環境・機体依存になるため禁止です。env.Sys または仮想パスを使用してください"
      - p: '^exec\.Command(Context)?$'
        msg: "直接 exec.Command を呼ばず、env.Cmd を使用してください"
      - p: '^(math/rand|crypto/rand)\.'
        msg: "直接乱数を生成せず、env.Id.NewID() を使用してください"

issues:
  exclude-rules:
    # outside 基盤と、adapters 層の本番実装 (libs/platform/*/adapter_real.go) と 入口の main.go (apps/*・tools/*) のみ許可
    - path: ^(libs/outside/go/|libs/platform/[^/]+/adapter_real\.go$|apps/[^/]+/main\.go$|tools/[^/]+/main\.go$)
      linters:
        - forbidigo
```

---

## 4. Dart / Flutter 禁止構文の検査（`outside_lint`）

Dart は標準アナライザだけではメソッド単位の禁止ができないため、`outside` の `lint/dart`（`outside_lint`、`package:analyzer` の構文木を読む）を Nx の `lint` ターゲットで回す。grep の正規表現より誤検出が無く、`DateTime.now`・`Platform.environment`・`Process.run` 等・`File(...)`・`Directory(...)`・`Directory.systemTemp`・`HttpClient(...)`・`Timer(...)` を行番号つきで名指す。

```bash
# libs/outside/flutter と *_real.dart と main.dart を除く全 Dart を検査する
dart run outside_lint:check apps libs/platform
# exit 0 = 違反なし / 1 = 違反あり / 3 = 読めない file（判定不能）
```

`*_real.dart` と `main.dart` は 検査の対象 dir から外すか 個別に除く。実機パスのハードコード（`/Users/` 等）は 同じ検査に足す。

## 4.1 adapters 層の命名と 3 file の検査（`outside` の `lint/naming`）

- 置き場 = `outside` repo の `lint/naming/`（Go、module `github.com/aimglow/outside/lint/naming`）。
  - `rules.go` = 規則の正本（機能の語の表・runtime の 4 つ・名前の正規表現・runtime ごとの 3 file の名前）。docs は ここを指すだけで 語の表を写さない。
  - `check.go` = 判定（dir 名と各 dir の file 名の一覧を値で受け 違反の一覧を値で返す。disk を読まない）。test は この判定を 値の表で確かめる（`outside` の中だけで回す）。
  - `main.go` = 入口（`--root` の dir を読んで判定へ渡す）。
- 検査すること（`--root` の直下の dir ごと）: 名前が `<機能>-<依存先>-<runtime>` か・3 file（Go = `interface.go`・`adapter_real.go`・`adapter_mock.go` / TS = `interface.ts`・`real.ts`・`mock.ts` / Dart = `interface.dart`・`adapter_real.dart`・`adapter_mock.dart`）が在るか。
- 出力 = 違反 1 件ずつ `libs/platform/<名前>: <何が違うか>`。exit = 0 違反なし / 1 違反あり / 3 読めない（判定不能）。
- 各 project = `.ccark-prepush.toml` に 1 段として宣言する: run = `go run github.com/aimglow/outside/lint/naming@<tag> --root libs/platform`、reads = `libs/platform/**` と tag。違反は push の前と便で赤 = land できない。`libs/platform` と tag が変わらなければ 段は前の PASS に当たり 回らない。各 project で この検査を書かない。

## 5. Nx キャッシュ設定の最適化（`nx.json`）

マシン環境の違いで Nx のタスクハッシュが変わるのを防ぐため、`namedInputs` から環境変数依存を排除し、決定論的な入力のみをハッシュ対象にします。

```json
{
  "$schema": "./node_modules/nx/schemas/nx-schema.json",
  "namedInputs": {
    "default": ["{projectRoot}/**/*", "sharedGlobals"],
    "production": [
      "default",
      "!{projectRoot}/**/?(*.)+(spec|test).[jt]s?(x)?(.snap)",
      "!{projectRoot}/**/*_test.go",
      "!{projectRoot}/test/**/*_test.dart",
      "!{projectRoot}/CLAUDE.md"
    ],
    "sharedGlobals": [
      "{workspaceRoot}/nx.json",
      "{workspaceRoot}/.golangci.yml",
      "{workspaceRoot}/eslint.config.js"
    ]
  },
  "targetDefaults": {
    "test": {
      "cache": true,
      "inputs": ["default", "^production"],
      "configurations": {
        "ci": {
          "ci": true
        }
      }
    },
    "lint": {
      "cache": true,
      "inputs": ["default", "sharedGlobals"]
    }
  }
}
```
