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
