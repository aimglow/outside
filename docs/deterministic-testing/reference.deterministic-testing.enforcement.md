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

`libs/outside/**` および `libs/platform/*/src/real.ts` **以外**のすべてのファイル（ビジネスロジック、`mock.ts`、Unit テスト）に適用します。

```javascript
// eslint.config.js（非決定要素・生IO・実機絶対パス禁止ルール）
{
  files: ['**/*.ts', '**/*.tsx'],
  ignores: [
    'libs/outside/**',
    'libs/platform/*/src/real.ts',
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
          { name: 'next/headers', message: 'adapters 層 (例 libs/platform/authn-cookie-next) を経由してください' },
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

## 3. Go 禁止構文の検査（`outside` の `lint-go`）

- 禁じる呼び出しの一覧の正本 = `outside` の部品 `lint-go` の `golangci.yml`（各 project では `libs/outside/lint-go/golangci.yml`）。この文書に一覧を写さない。
  禁じる物 = 時計（`time.Now` 等 → `Clock`）・環境変数（→ `Env`）・機体と process の値（`os.Hostname`・`os.Getpid`・`os.UserHomeDir` 等 → `SysInfo`）・
  乱数（`math/rand`・`math/rand/v2` → `IdGen`、`crypto/rand` の `Read`・`Reader`・`Text`・`Prime` → `Random`。`crypto/rand.Int` は 読み手を受けるので `Random` を渡せば通る）・子 process（→ `Proc`）・system の一時置き場（→ `Workdir`）・file（→ `FS`）。
- 除外は 取り込む project の側の設定に置く（`lint-go` には入れない）。project の `.golangci.yml` は `lint-go` の `forbidigo` の設定を そのまま持ち（`lint-go` を取り込み直したら 合わせる）、除外だけを足す:

```yaml
# .golangci.yml (project の根。forbidigo の設定は libs/outside/lint-go/golangci.yml と同じ)
version: "2"
linters:
  default: none
  enable:
    - forbidigo
  settings:
    forbidigo:
      analyze-types: true
      forbid:
        # libs/outside/lint-go/golangci.yml の forbid を そのまま置く
  exclusions:
    rules:
      # outside から取り込んだ写し (libs/outside/ の全部品) と、adapters 層の本番実装 (libs/platform/*/adapter_real.go) と 入口の main.go (apps/*・tools/*) のみ許可
      - path: ^(libs/outside/|libs/platform/[^/]+/adapter_real\.go$|apps/[^/]+/main\.go$|tools/[^/]+/main\.go$)
        linters:
          - forbidigo
```

- `analyze-types: true` の下では `pattern` は 型から引いた「package の名前.名前」に、`pkg` は import path に照らされる。`math/rand` と `crypto/rand` は 名前が同じ `rand` ゆえ `pkg` で区切る（`pattern` に import path を書いても当たらない）。

---

## 4. Dart / Flutter 禁止構文の検査（`outside_lint`）

Dart は標準アナライザだけではメソッド単位の禁止ができないため、`outside` の `lint-dart`（`outside_lint`、`package:analyzer` の構文木を読む）を Nx の `lint` ターゲットで回す。grep の正規表現より誤検出が無く、`DateTime.now`・`Platform.environment`・`Process.run` 等・`File(...)`・`Directory(...)`・`Directory.systemTemp`・`HttpClient(...)`・`Timer(...)` を行番号つきで名指す。

```bash
# libs/outside と *_real.dart と main.dart を除く全 Dart を検査する
dart run outside_lint:check apps libs/platform
# exit 0 = 違反なし / 1 = 違反あり / 3 = 読めない file（判定不能）
```

`*_real.dart` と `main.dart` は 検査の対象 dir から外すか 個別に除く。実機パスのハードコード（`/Users/` 等）は 同じ検査に足す。

## 4.1 adapters 層の命名と 3 file の検査（`outside` の `lint-platform`）

- 置き場 = `outside` repo の部品 `lint-platform`。各 project では `libs/outside/lint-platform/`（`sync-outside.sh lint-platform <tag>` で置く。外の module は vendor で同梱 = 網を使わずに回る）。
  - `naming/rules.go` = 規則の正本（runtime の 4 つ・全 project 共通の機能の語・中身を表さない語・runtime ごとの 3 file の名前）。docs は ここを指すだけで 語の表を写さない。
  - `naming/check.go` = 判定（値を受け 違反と警告を値で返す。disk を読まない）。`naming/scaffold.go` = 雛形（名前と 3 file の中身を値で返す）。
  - `main.go` = 入口（`check` と `scaffold`）。
- adapters の置き場は 宣言 `platform_dir`（書かなければ `libs/platform`。repo で `libs/platform` が別の用途なら `""` = 持たない か 別の dir）。`--root` を省けば それを使い（repo の根で撃つ）、宣言と違う `--root` は 判定不能。`""` の check は 検査せずに名乗って 0。
- 決めるのは 3 つだけ: ①直下の dir 名が `<機能>-<依存先>-<runtime>` ②直下に dir 以外を置かない ③各 lib に runtime の 3 file（直下か `src/` か `lib/`）。lib の中の sub dir・3 file 以外の file・設定の file は見ない（言語とツールの慣習に任せる）。
- project 固有の機能の語 = repo の根の `outside-platform.toml` に `capabilities = ["pay", ...]`。中身を表さない語（`common`・`util`・`shared`・`helper`・`misc` 等）は宣言できない（判定不能 = exit 3）。
- 同じ runtime で 同じ依存先が 2 つの機能に在れば 警告（止めない）。
- 出力 = `違反 libs/platform/<名前>: <何が違うか>` / `警告 …`。exit = 0 違反なし / 1 違反あり / 3 読めない（判定不能）。
- 撃ち方（repo の根で。check・scaffold・tests で共通）: 先に `GOWORK=off go -C libs/outside/lint-platform build -o ../../../.work/bin/lint-platform .` で道具を作り（`.work/` は .gitignore に入れる）、`.work/bin/lint-platform <sub> …` で撃つ。
  - `GOWORK=off` = repo の根に `go.work` が在っても lint-platform 自身の module と vendor で build する（`go.work` が `libs/outside/lint-platform` を包まないと build できないため）。
  - `go run` で撃たない: `go run` は 道具の exit 3（判定不能）を exit 1 に潰す（実測）= 判定不能が違反に見える。build した道具は 0・1・3 をそのまま返す。
  - 別の撃ち方（例 go.mod の tool）を取る project は 判定不能の宣言で exit 3 が返る事を 1 度確かめる（陽性対照）。
- lib を作る = `.work/bin/lint-platform scaffold --root libs/platform --vocab outside-platform.toml <機能> <依存先> <runtime>`（名前が通らなければ何も作らない）。
- 各 project = push の前の検査に 1 段として宣言する: run = `GOWORK=off go -C libs/outside/lint-platform build -o ../../../.work/bin/lint-platform . && .work/bin/lint-platform check --root libs/platform --vocab outside-platform.toml`、reads = `libs/platform/**`・`libs/outside/lint-platform/**`・`outside-platform.toml`。違反は push の前と便で赤 = land できない。読む物が変わらなければ 段は前の PASS に当たり 回らない。各 project で この検査を書かない。

## 4.2 本物の外部に触れる test の置き場の検査（`outside` の `lint-platform` の `tests`）

- 決まりの正本 = `reference.deterministic-testing.test-classification.md` §4（本物の外部に触れる test は `outside` と adapters の中だけ。apps・tools には 印を付けても置かない）。この検査は それを機械で止める。
- 置いてよい場所 = adapters の置き場（`outside-platform.toml` の `platform_dir`。書かなければ `libs/platform`、`""` なら持たない）の下と 同じ宣言の `adapter_dirs = ["..."]`（repo の根からの相対 path。`.`・空・絶対 path・根の外は 判定不能）。
- 落とす物（置いてよい場所の外の test の file）:
  - integration の印を持つ test（Go = `//go:build` に `integration`、TS = `*.integration.spec.*`・`*.integration.test.*`、Dart = `@Tags([... 'integration' ...])` か `tags:` の `'integration'`）。
  - 印に依らず 本物の口を作る物を使う test（outside の本物 = Go の `outside.New*`・`RunTool`、TS の `createReal*`、Dart の `Real*(…)`・`OutsideSet.real`。adapters の本物 = `adapter_real.go`・`src/real.ts`・`adapter_real.dart` が公開する名前）。mock だけを使う test は通す。
- 見えない形（反射・文字列で組む import・動的 import・tsconfig の paths の別名・置き場の外の helper を経由する形）は 書く時の審査が持つ（`design.outside.yaml` の `lint.platform.tests.not_checked`）。
- 出力 = `違反 <path>[:<行>]: …` / `判定不能 <path>: …`。exit = 0 違反なし / 1 違反あり / 3 読めない（判定不能）。
- 各 project = push の前の検査に 1 段として宣言する: run = `GOWORK=off go -C libs/outside/lint-platform build -o ../../../.work/bin/lint-platform . && .work/bin/lint-platform tests --repo .`（撃ち方の決まり = §4.1）、reads = `**/*_test.go`・`**/*.spec.*`・`**/*.test.*`・`**/*_test.dart`・`libs/platform/**`・`adapter_dirs` の下・`**/go.mod`・`**/pubspec.yaml`・`libs/outside/lint-platform/**`・`outside-platform.toml`。

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
