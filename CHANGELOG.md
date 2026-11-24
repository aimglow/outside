# CHANGELOG


## v0.1.0 (2026-11-23)

最初の版。

- 足した口
  - Go (`github.com/aimglow/outside/go`、mock = `outsidemock`): Clock・Env・FS・Git・Proc・Workdir、ツールの入口 RunTool (使う環境の宣言 `outside.toml` を確かめ 外れていれば名乗って落ちる)。
  - TypeScript (`@aimglow/outside`、mock = `@aimglow/outside/mock`、Angular の口 = `@aimglow/outside/angular`): Clock・Http・Storage・Config。
  - Dart (`outside`、mock = `package:outside/mock.dart`): Clock・Http・Storage・Config (Flutter の iOS と Android 用。web は対象外)。
- 足した project: なし (最初の版)。
- 利用する project がやる事: まだ無い。組み込み方は kifune で確かめてから CLAUDE.md に書く。
- まだ無い物: lint の設定 (`lint/`)。

## v0.2.0 (2026-11-23)

- 足した物: Go の lint の設定 `lint/go/golangci.yml` (golangci-lint 2.x の forbidigo)。
  外を直に呼ぶ code (時計・env・機体・子 process・system の一時置き場・file) を落とし 口を通す code を通す。
  確かめ = `lint/go` の test が 落とす形と通す形の fixture に 実の golangci-lint を掛ける (TC-OUT-LINT-GO)。
- 足した project: なし。
- 利用する project がやる事: 今は無い (取り込み方は kifune で確かめてから CLAUDE.md に書く)。
- まだ無い物: TypeScript・Dart の lint の設定。

## v0.3.0 (2026-11-23)

足した project = kifune (#244 の tools/dbgate の付け替え)。

- 変えた: 判定不能の exit の番号を 2 から 3 にした (`ExitUndetermined = 3`)。kifune の段と ccark の検査の口が 3 = 判定不能のため。
- 足した: Git に `Resolve(ref)`・`ListAt(rev, dir)`・`ReadAt(rev, path)`。
- 足した: Workdir に `FS(path)` (作った作業場を根にした FS)。
- 足した: `workdir_root` に `$NAME/<相対 path>`。
- 直した: mock の文書の語を mock に揃えた。
- 決めた: 版の番号の意味 (0.x = どの project の main にも まだ入っていない間 / 1.0.0 = 最初の project の main に入る時)。

利用する project がやる事: 取り込む版を v0.3.0 に上げる (0.x の間は 意味を変える変更も MINOR = 版の意味は 設計書 versioning.meaning)。`ExitCode` の 2 を判定不能として読んでいる所は 3 に直す。Git・Workdir を自分で実装している型は 足した method を実装する (outside の mock を使っていれば 不要)。今 取り込んでいるのは kifune の branch だけ = kifune の #244 で 上げる。

## v0.4.0 (2026-11-23)

足した project = なし (全 project が取り込む lint を そろえた)。

- 足した: TypeScript の lint の設定 `lint/ts/eslint.config.js` (ESLint の flat config。Date.now・引数なしの new Date・setTimeout・setInterval・process.env・fetch・localStorage・sessionStorage・window / globalThis 越しの同じ物・fs と child_process の import を落とす)。
- 足した: Dart の lint `lint/dart` (`dart run outside_lint:check <dir>`。DateTime.now・Platform.environment・Process.run 等・File(...)・Directory(...)・Directory.systemTemp・HttpClient(...)・Timer(...) を 行で名指す。exit 0 / 1 / 3 = 判定不能)。

利用する project がやる事: 取り込む版を v0.4.0 に上げ、TypeScript は project の eslint.config.js に取り込み、Dart は 段で `dart run outside_lint:check lib` を回す。Go の lint は v0.2.0 から在る。

## v0.5.0 (2026-11-23)

足した project = kifune (#244 の tools/secretprobe の付け替え)。

- 足した: Proc の `Cmd.Stdin` (子 process の標準入力。秘密を disk にも argv にも載せずに渡す)。mock の Proc は 起こされた Cmd を `Cmds()` で返す (Stdin・Env・Dir まで test から読める)。
- 足した: `IOFS(FS)` (FS を io/fs.FS として読ませる)。kifune が tools/internal/iofs に作った口を 部品へ移した。
- 足した: lint の module の tag `lint/go/v0.5.0` (取り込む project は 版の代わりの仮の番号 (pseudo-version) を使わずに済む)。

利用する project がやる事: go.mod の outside を v0.5.0 に上げ、lint の module を `lint/go/v0.5.0` で取る。kifune は tools/internal/iofs を `outside.IOFS` に置き換える。


## v0.6.0 (2026-11-24)

足した: adapters 層 (`libs/platform`) の 名前と構成の検査 `lint/naming` (module `github.com/aimglow/outside/lint/naming`)。
規則の正本 = `lint/naming/naming/rules.go`。見る事 = 直下の dir 名が `<機能>-<依存先>-<runtime>` か・runtime ごとの 3 file (interface / real / mock) が直下か src・lib に在るか・lib の直下に src・lib・test・testdata 以外の dir が無いか・直下に dir 以外の file が無いか。
exit = 0 違反なし / 1 違反あり / 3 読めない (判定不能)。

利用する project がやる事: `.ccark-prepush.toml` に 1 段として宣言する。run = `go run github.com/aimglow/outside/lint/naming@v0.6.0 --root libs/platform`、reads = `libs/platform/**`。各 project で この検査を書かない。

## 部品ごとの版へ (2026-11-24)

版は 部品ごとの tag `<部品>/vX.Y.Z` で出す (部品の表 = `docs/developer/design.outside.yaml` の components)。
各 project は `scripts/sync-outside.sh <部品> <tag>` で 部品ごとに版を選び 自分の repo の中へ置く (検査のたびに網から取らない)。
版は `scripts/release.sh <部品> <major|minor|patch>` で切る。

- `lint/go`・`lint/ts`・`lint/dart` → `lint-go/`・`lint-ts/`・`lint-dart/` (中身は同じ)。最初の版 = 各 `v0.1.0`。
- `lint/naming` → `lint-platform/` (`lint-platform/v0.1.0`)。v0.6.0 の `lint/naming` から変えた所:
  - 検査は `check`、雛形の作成は `scaffold` (名前が通らなければ何も作らない)。
  - 決めるのは 3 つだけ (直下の dir 名 / 直下に dir 以外を置かない / runtime の 3 file)。lib の中の sub dir は見ない (v0.6.0 の「置いてよい dir」の規則は外した)。
  - project 固有の機能の語は repo の根の `outside-platform.toml` の `capabilities` に宣言する。中身を表さない語 (common・util・shared 等) は宣言できない。
  - 同じ runtime で同じ依存先が 2 つの機能に在れば 警告 (止めない)。
  - 外の module (BurntSushi/toml) は vendor で同梱。
  - 各 project の段 = `go -C libs/outside/lint-platform run . check --root ../../platform --vocab ../../../outside-platform.toml`。
- `docs` (`docs/deterministic-testing/`) = `docs/v0.1.0`。各 project の lint の除外を `libs/outside/` の全部品へ広げた (enforcement §3・§4)。
- `ts/`・`dart/` を `next/`・`angular/`・`flutter/` に分けるのは まだ (分けた時に 各部品の最初の版を出す)。`go/` は `go/v0.6.0` のまま。

## docs/v0.2.0・lint-platform/v0.1.0 のまま (2026-11-24)

- docs: 運用 (`reference.deterministic-testing.operations.md`) を 部品ごとの版の形へ書き直した (版の正本の表・版の発行・取り込み・依頼の流れ)。門番規約・製造規約 §4・目次の木を同じ形へ。adapters 層の lib を作る基準 (依存先に固有の取り決めを読み書きする時だけ。起こして終了コードを見るだけなら Proc) を adapters.md §0 に足した。
- 版を memory・CLAUDE.md・issue・agent の定義に書き写さない (版の正本 = tag・CHANGELOG.md・各 project の .outside-version)。
- scripts/release.sh: 打つ前に origin の tag を取り直す。部品の形でない tag が在れば止まる。部品の形でない古い tag (v0.1.0〜v0.6.0・lint/go/*・lint/naming/v0.6.0) は消した (参照している project が無い事を確かめた上で)。
- 利用する側がやる事: docs/v0.2.0 を取り込む。他の部品は変わらない。

## scripts・docs/v0.3.0 (2026-11-24)

- scripts/sync-outside.sh: 部品の test (Go の `*_test.go`・`testdata/`、TS の `*.test.*`・`*.spec.ts`、Dart の `*_test.dart`・`test/`) を写さない。写すと 取り込んだ project の test の検査 (ccark testguard 等) が 部品の test を その project の test として咎める (kifune #244 で実測)。
- 利用する側がやる事: outside の main の `scripts/sync-outside.sh` を写し直し、置いた部品を 同じ tag で取り込み直す (部品の test が消える)。docs は docs/v0.3.0。

## go/v0.7.0 (2026-11-24)

- 作業場の置き場 (outside.toml の workdir_root) が repo の中の相対 path なら 無ければ作る。$NAME で指す置き場は 今までどおり 無ければ判定不能 (exit 3)。
  理由: 隔離した環境 (ccark の land の段は CCARK_DATA を 空の使い捨ての dir にする) で $CCARK_DATA/<dir> を置き場にした tool が 置き場が無く止まった (kifune #244 で実測)。
  repo の中なら 新しい clone でも 同じ形で回る。
- Base に EnsureWorkdir を足した (nil なら Workdir と同じ)。RunToolWith に 自前の Base を渡す側は 足さなくても 今までどおり動く。
- 利用する側がやる事: tool の workdir_root を repo の中の相対 path (例 ".work/<tool>"、.gitignore に入れる) にする。$CCARK_DATA の下を置き場にしない (ccark が約束した置き場ではない)。

## docs/v0.4.0 (2026-11-24)

- test の書き方の規則の正本を ccark の `docs/conventions/test-writing.md`（全 project 共通、R0〜R7・V1〜V3）に一本化した。test-classification.md と app-rules.md §1 は 規則を自前で持たず 分類 A / B / C を その規則番号へ対応させるだけにした（規則集を 2 つにしない）。
- 利用する側がやる事: docs/v0.4.0 を取り込む。各 project の完了の条件に test-writing.md §6 の 4 条件（testguard-writing の段・棚卸し・Nx の入力の宣言・陽性対照）を入れる。

## docs/v0.5.0 (2026-11-24)

- operations.md §6 = 各 project の付け替えの完了の条件 (6 つの AND) を足した。issue の DoD は この節を指して書く。


## scripts・docs/v0.6.0 (2026-11-24)

- scripts/sync-outside.sh: 写した中身の指紋を `.outside-version` に書く (1 行目 = tag のまま / 2 行目 = `fingerprint sha256:<全体>` / 続く行 = `file <sha256> <path>`)。
  `sync-outside.sh --check <部品>` が 網に行かずに 置き場を数え直して照らす。exit 0 = 一致 / 1 = 違う (違う・増えた・消えた file を名指す) / 3 = 判定不能 (置き場・.outside-version・指紋の行が無い)。
  理由: 写しを編集しない決まりを 合流後の木で 機械で確かめ 古い branch の merge で写しが書き換わるのを land の段で止める (kifune #244 の頼み)。
- docs: 運用 (operations.md §2・§4) に 指紋と `--check` を足した。版は `.outside-version` の 1 行目 (`head -1`)。
- 利用する側がやる事: outside の main の `scripts/sync-outside.sh` を写し直し、置いた部品を 同じ tag で取り込み直す (指紋が書かれる。指紋の無い写しは `--check` が判定不能 = exit 3)。land の前の段に 部品ごとに `./scripts/sync-outside.sh --check <部品>` を置く。docs は docs/v0.6.0。

## next/v0.1.0・angular/v0.1.0・flutter/v0.1.0 (2026-11-24)

`ts/` を `next/` と `angular/` に、`dart/` を `flutter/` に分けた (部品 = runtime ごと、design.outside.yaml の components)。各部品の最初の版。

- 口の組 (3 部品とも同じ、design.outside.yaml の ports.frontend_and_mobile): Clock (now・setTimeout・clearTimeout)・Http (request)・Storage (get・set・remove)・Config (get)・IdGen (newId)。
  - 足した: IdGen。real = UUID v4 (TS は crypto.randomUUID、Dart は Random.secure)。mock = id-0001・id-0002 … (mock ごとに 1 から)。set (createRealSet・createMockSet / OutsideSet) に idGen を足した。
- next (`@aimglow/outside-next`、mock = `@aimglow/outside-next/mock`): Node と Next.js。Config の real は 渡した値 → process.env の順。dependencies は空。
- angular (`@aimglow/outside-angular`、mock = `@aimglow/outside-angular/mock`): browser。Config の real は 渡した値だけ (process を見ない)。`@angular/core` に依存しない = `ts/` に在った `provideOutside`・`OUTSIDE_*` の InjectionToken は持たない (DI の token は 各 project の adapters 層が持つ)。
- flutter (Dart の package 名は `outside` のまま、mock = `package:outside/mock.dart`): iOS と Android。dependencies は空 (`package:http` を外し Http の real は `dart:io` の HttpClient。口の形 HttpRequest・HttpResponse は同じ)。Config の real は 渡した値 (build 時) → 環境変数の順。
- 確かめ (TC-OUT-RUNTIME-DEPS): 各部品の依存の宣言と import が runtime の標準の外を持たない事を test が source から読んで確かめる。
- 利用する側がやる事: `./scripts/sync-outside.sh next next/v0.1.0` (Next.js)・`angular angular/v0.1.0` (Angular)・`flutter flutter/v0.1.0` (Flutter) で取り込む。`ts/` の `@aimglow/outside` を参照していた所は `@aimglow/outside-next` か `@aimglow/outside-angular` へ。Angular の DI の token は adapters 層に置く。Dart の `package:http` を前提にした所は無い (口の形は同じ)。

## go/v0.8.0・lint-go/v0.2.0・docs/v0.7.0 (2026-11-24)

- go (0.x ゆえ 意味を変える変更も MINOR):
  - 消した: 口 Git (`NewGit`・`Set.Git`・`Base.Git`、mock の `outsidemock.NewGit`・`GitValues`・`Pair`)。特定ツールの名前の口は outside に置かない (REQ-OUT-NO-TOOL-NAME)。
  - 足した: 口 SysInfo (`Hostname() (string, error)`・`HomeDir() (string, error)`・`Pid() int`・`IsTTY() bool`。real = `outside.NewSysInfo()`、mock = `outsidemock.NewSysInfo(outsidemock.SysInfoValues{...})` = 渡していない Hostname・HomeDir は誤り・Pid 0・IsTTY false)。
  - 足した: 口 IdGen (`NewID() string`。real = `outside.NewIdGen()` = crypto/rand の UUID v4、mock = `outsidemock.NewIdGen()` = id-0001 から呼んだ順・並行に呼んでも安全)。`Set` と `Base` に `SysInfo`・`IdGen`。
  - 変えた: RunTool の `requires_git_worktree` は git の program を起こさず 宣言の dir から上へ `.git` (dir でも file でも) を探す (PATH に git が無くても同じ結果)。`RunToolWith` は `Base.Worktree func() (string, error)` で確かめる (nil か誤りなら『git の木』が足りないと名乗り exit 3)。
  - そのまま: Proc の `Cmd.Stdin`。依存は 標準と BurntSushi/toml だけ。
- lint-go: 禁止を足した。`os.Hostname`・`os.Getpid`・`os.Getppid`・`os.Getwd`・`os.UserHomeDir`・`os.UserConfigDir`・`os.UserCacheDir` (→ SysInfo)、`math/rand`・`math/rand/v2`・`crypto/rand` の全て (→ IdGen。`pkg` で import path に照らす = 名前だけ rand の自作 package は落とさない)。除外 (exclude-rules) は lint-go に入れない。
- docs: enforcement.md §3 を lint-go の golangci.yml を正本として指す形にし 除外だけを project 側に置く例 (golangci 2.x) にした。architecture.md・門番規約・outside-spec.md の口と依存の範囲を 実物 (next・angular・flutter の分割、REQ-OUT-RUNTIME-DEPS) に合わせた。
- 利用する側がやる事:
  - go: Git を使っていた所は 各 project の adapters 層 `libs/platform/vcs-git-go` (interface / real / mock) へ移し git は `Proc` で起こす (outside.toml の programs に git を宣言する)。`outsidemock.NewGit` を使う test は その adapters の mock に替える。`Set`・`Base` を自前で組む所は `SysInfo`・`IdGen` を足す。`Base` で `requires_git_worktree` を使う所は `Base.Worktree` を渡す。
  - lint-go: 上げると 今まで通っていた code が落ちうる。先に go を v0.8.0 へ上げ、os.Hostname 等は SysInfo、乱数は IdGen を通す。入口の main.go・adapter_real.go は project 側の除外 (enforcement.md §3) に在る事を確かめる。
  - docs: docs/v0.7.0 を取り込む。
