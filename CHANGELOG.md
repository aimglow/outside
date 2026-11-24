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

## scripts (2026-11-24)

- scripts/sync-outside.sh: 置き場の入れ替えで 前の写しを失わない。新しい写しを用意し終えてから名前を付け替え、入れる mv が失敗したら 前の写しを戻して名乗る (戻せなければ 一時の dir を消さず その場所を名乗る)。kifune #244 の審査の指摘。
- bash のまま置く理由 = 部品を repo へ入れる最初の口ゆえ 部品の無い repo で動く必要がある・設定の file を読まない (design.outside.yaml の consume.why_script)。
- 利用する側がやる事: outside の main の scripts/sync-outside.sh を写し直す (部品の版は変わらない)。

## go/v0.9.0 (2026-11-25)

足した口 (各 project の本体の code が直に呼んでいた物の受け先):
- Clock: `NewTimer(d) Timer` (`C()`・`Stop() bool`・`Reset(d) bool`)・`NewTicker(d) Ticker` (`C()`・`Stop()`)・`After(d)`。real は time を包む。mock は Advance・Sleep で 期限を過ぎた物を 期限の順 (同じ期限は掛けた順) に発火し その期限の時刻を送る (buffer 1・満ちていれば落とす = time と同じ。実時間を待たない)。経過 (Since・Until) は `Now()` から引く (口に足さない)。
- FS: `Open` (io.ReadCloser で stream に読む)・`AppendFile` (無ければ作り 末尾に足す)・`MkdirAll`・`Lstat` (`FileInfo.IsSymlink` を足した)・`Rename` (両方の path が根の中。移し先の file は置き換え、dir・根は誤り)・`RemoveAll` (根は誤り・無い path は nil)。real は os.Root の上 (根の外を指す symlink は止まる)。mock FS も同じ誤りを返す。`IOFS(FS)` は FS.Open で読む。
- Proc: `LookPath(name)`。宣言した program だけ (無い名前は ErrUndeclaredProgram、実行 file が無ければ exec.ErrNotFound)。mock は `outsidemock.NewProc(...).WithPaths(map[string]string{...})`。
- RunTool の Set (limitFS・limitProc) も 足した method を宣言の範囲に絞る。
- 足さない: Env の全部の一覧 (Environ)。宣言した名前だけを読ませる絞りを迂回させないため。子 process の環境は Proc (親の環境 + Cmd.Env)。

利用する側がやる事: outside.FS・Clock・Proc を自前で実装している型は 足した method を実装する (outside の mock を使っていれば不要)。`time.NewTimer`・`time.NewTicker`・`time.After`・`os.Open`・`os.OpenFile(O_APPEND)`・`os.MkdirAll`・`os.Lstat`・`os.Rename`・`os.RemoveAll`・`exec.LookPath` を直に呼ぶ所は Set の Clock・FS・Proc へ移す。`time.Since(t)` は `clock.Now().Sub(t)`。

## go/v0.10.0・lint-go/v0.3.0・docs/v0.8.0 (2026-11-25)

- go: 口 Random (`Read(p []byte) (int, error)` = io.Reader と同じ形。crypto の API へ そのまま渡せる) を足した。real = `outside.NewRandom()` (crypto/rand)。mock = `outsidemock.NewRandom(seed)` (同じ種・同じ呼び順なら同じ byte 列。分けて読んでも続けて読んでも同じ列。読んだ長さを記録。`WithErr(err)` で誤りを返させる)。`Set`・`Base` に `Random` (宣言で絞らない)。
- lint-go: crypto/rand は `Read`・`Reader`・`Text`・`Prime` だけを落とす (→ Random)。`rand.Int` は 読み手を受けるので Random を渡せば通る (今まで落ちていた形を通すだけ = 赤を増やさない)。math/rand・math/rand/v2 は 今までどおり全て落とす (→ IdGen)。
- docs: enforcement.md §3・app-rules.md・non-deterministic.md の 乱数の受け先を IdGen と Random に分けて書いた。
- 注意 (Go 1.26): `crypto/ecdh` の `GenerateKey`・`crypto/rand.Prime` 等の鍵の生成は 渡した読み手を無視し 常に安全な乱数を使う (GODEBUG=cryptocustomrand=1 の時だけ使う)。mock の Random を渡しても 鍵は決まらない。鍵を test で決めたい処理は Random で読んだ byte から作る (例 `curve.NewPrivateKey(b)`、X25519 は 32 byte)。
- 利用する側がやる事: go を v0.10.0・lint-go を v0.3.0 に上げ、crypto/rand.Read・Reader を Set.Random へ、rand.Int(crand.Reader, max) を rand.Int(set.Random, max) へ移す。`Set`・`Base` を自前で組む所は Random を足す。

## next/v0.2.0・angular/v0.2.0・flutter/v0.2.0 (2026-11-25)

- 足した: 口 Random (`read(buf)` = 渡した byte 列 (TS = Uint8Array、Dart = Uint8List) を全部埋める。長さは呼び手が決める = trace-id 16 byte・span-id 8 byte 等)。
  - real: TS = `createRealRandom()` (crypto.getRandomValues。65536 byte を超える buf は分けて埋める。crypto が無ければ名乗って throw)、Dart = `RealRandom()` (Random.secure)。
  - mock: TS = `createMockRandom(seed)`、Dart = `MockRandom(seed)`・`MockRandom.text(seed)`。種と呼び順で決まる列 (分けて読んでも続けて読んでも同じ。3 部品で同じ種から同じ列)。`calls` = 読んだ長さの順。`withErr(err)` で以後の read が投げる (undefined・null で戻る)。
  - set: `OutsideSet` に `random`。TS の `createMockSet` の options に `seed` (既定 `DEFAULT_MOCK_RANDOM_SEED`)。
- 利用する側がやる事: `createRealSet`・`createMockSet`・`OutsideSet.real` を使う所は 何もしなくてよい。set を自分で組む所は `random` を足す (Dart の `OutsideSet(...)` は `random:` が必須)。乱数の byte を作る code は crypto を直に呼ばず `set.random.read(buf)`。Dart で `dart:math` と `package:outside/outside.dart` を両方 取り込む file は `Random` の名が衝突する = `import 'dart:math' as math;` か `hide Random`。
## go/v0.11.0・lint-go/v0.4.0 (2026-11-25)

- go: FS に `Symlink(target, link)`・`Readlink(link)`。在る symlink は 同じ dir に一時の名前で作ってから付け替える (途中で link が無い時を作らない)。link が file・dir なら誤り、親の dir が無ければ fs.ErrNotExist。根の外を指す target も 作るだけなら通す (辿ると誤り)。mock の FS も symlink の節を持ち 根の中に解ける target だけを辿る (Lstat・Remove・RemoveAll・Rename・Readlink は link 自身)。
- go: Proc に `Cmd.KillGroup`・`Cmd.KillGrace` (0 なら 5s)・`Result.Killed`。KillGroup の子は 新しい process group で起き ctx の終わりで group (孫まで) へ SIGTERM、猶予の後に SIGKILL。ctx の終わりで止めた時は KillGroup に依らず `Killed = true` で 誤りは ctx の誤りを包む (`errors.Is(err, context.Canceled)` 等で判る)。unix でない OS の KillGroup は誤り。子が自分で終えたのに孫が pipe を握って残る時は 猶予の後に exec.ErrWaitDelay を包む誤り (Killed = false)。mock は `outsidemock.NewProc(...).WithFunc(name, func(ctx, cmd) (outside.Result, error))` で ctx を受ける子を表せ Cmds() で KillGroup・KillGrace を読める。
- lint-go: `os.Symlink`・`os.Readlink`・`os.Link` を落とす (→ FS)。今まで通っていた code が落ちうる。
- 利用する側がやる事: outside.FS を自前で実装している型は Symlink・Readlink を足す。ctx の誤りを `==` で比べる所は `errors.Is` へ。孫まで止めたい子は `Cmd{KillGroup: true, KillGrace: ...}`。lint-go/v0.4.0 は go/v0.11.0 の後に上げる (symlink を FS へ移してから)。

## lint-platform/v0.2.0・docs/v0.9.0 (2026-11-25)

- lint-platform: 検査 `tests` を足した (`go -C libs/outside/lint-platform run . tests --repo ../../..`)。test-classification.md §4 (本物の外部に触れる test は outside と adapters の中だけ) を機械で止める。置いてよい場所 (`libs/platform/**` と `outside-platform.toml` の `adapter_dirs`) の外の test について:
  - integration の印 (Go の `//go:build integration`・TS の `*.integration.spec.*`・Dart の `@Tags(['integration'])`) を持てば 落とす。
  - 印に依らず 本物の口を作る物 (outside の `New*`・`RunTool`・`createReal*`・`Real*(…)`・`OutsideSet.real`、adapters の `adapter_real.go`・`src/real.ts`・`adapter_real.dart` が公開する名前) を使えば 行つきで落とす。mock だけの test は通す。
  - exit 0 / 1 / 3 (判定不能 = 宣言か file が読めない)。kifune #244 の頼み。
- lint-platform: `outside-platform.toml` に `adapter_dirs` を書ける (check・scaffold・tests が同じ宣言を読む)。`.`・空・絶対 path・根の外は 判定不能。前の版の lint-platform は adapter_dirs を知らない key として 判定不能にする = 宣言を書くのは v0.2.0 を取り込んでから。
- docs: enforcement.md §4.2 に 検査と段の宣言の形を足した。
- 利用する側がやる事: lint-platform を v0.2.0 に上げ、push の前の検査に段を 1 つ宣言する (run と reads = enforcement.md §4.2)。libs/platform 以外に adapter の dir が在れば adapter_dirs に書く。違反は 本物の外部に触れる test を libs/platform/<lib> か 宣言した dir へ移すか mock だけで書き直す。

## lint-platform/v0.3.0・docs/v0.10.0 (2026-11-25)

- lint-platform: 共通の機能の語に `mail`・`push`・`storage`・`secret`・`pdf`・`trace` を足した (業務を含まない技術の語。業務の語 (例 inventory) は 共通に入れず 各 project の outside-platform.toml に宣言する = 門番規約 §1-2)。語を足すだけ = 今まで通っていた名前は そのまま通る。
- 決め (amatelas #1399 の問い):
  - runtime の語は 部品と 1 対 1 のまま。Node で動く TypeScript は Next.js でなくても `next` (Node だけで動く lib も `-next`)。`node` は足さない。
  - adapters 層に置くのは real が外 (通信・file・子 process・機体の値) を触る lib だけ。外を触らない部品 (形式の変換・計算・集計) は 業務の側の lib に置く (mock を作らず そのまま test する)。触る部分と触らない部分を持つ物は 触る部分だけを切り出す (adapters.md §0)。
- docs: architecture.md・adapters.md の lib の名前の例を lint-platform の規則に通る形 (db-postgres-go・vcs-git-go・container-podman-go・authn-cookie-next 等) に直した (規則の正本は rules.go)。adapters.md §0 に 上の 2 つの決めを足した。
- 利用する側がやる事: lint-platform を v0.3.0 に上げると 6 つの語を outside-platform.toml から外せる (宣言に残しても誤りではない)。docs は docs/v0.10.0。

## docs/v0.10.1 (2026-11-25)

- docs: enforcement.md・non-deterministic.md に残っていた 古い lib の名前 (adapters/auth-cookie・adapters/llm-gateway) を 規則に通る形の例へ直した (文の直しだけ)。
