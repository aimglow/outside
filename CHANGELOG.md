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

利用する project がやる事: 写しを v0.3.0 に差し替える (0.x の間は 意味を変える変更も MINOR = 版の意味は 設計書 versioning.meaning)。`ExitCode` の 2 を判定不能として読んでいる所は 3 に直す。Git・Workdir を自分で実装している型は 足した method を実装する (outside の mock を使っていれば 不要)。今 写しを持つのは kifune の branch だけ = kifune の #244 で 差し替える。
