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
