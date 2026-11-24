# outside

application とツールが 外 (時計・環境変数・file・子 process・作業場・機体の値・ID・通信・保存) を触る場所を
この部品 1 つに限るための 共通部品。全 project が同じ版を使う。

- 設計書: `docs/developer/design.outside.yaml`
- テスト仕様: `docs/developer/spec.outside.yaml`

## 置き場

| dir | 中身 |
|---|---|
| `go/` | Go の module (`github.com/aimglow/outside/go`、 mock = `outsidemock`) |
| `next/` | TypeScript の library・Node と Next.js 向け (`@aimglow/outside-next`、 mock = `@aimglow/outside-next/mock`。依存は `node:*` だけ) |
| `angular/` | TypeScript の library・browser 向け (`@aimglow/outside-angular`、 mock = `@aimglow/outside-angular/mock`。依存は browser の標準の Web API だけ。DI の token は 各 project の adapters 層が持つ) |
| `flutter/` | Dart の package・iOS と Android 向け (`outside`、 mock = `package:outside/mock.dart`。依存は `dart:*` だけ) |
| `lint-go/`・`lint-ts/`・`lint-dart/` | stack ごとの lint の設定 (外を直に呼ぶ code を落とす) |
| `lint-platform/` | adapters 層 (各 project の `libs/platform`) の 名前と構成の検査と 雛形の作成 |
| `scripts/` | `release.sh` (部品ごとに版を切る)・`sync-outside.sh` (project が部品を取り込む) |

部品ごとに版を出す (tag = `<部品>/vX.Y.Z`)。部品の表と 各 project の中の置き場 = `docs/developer/design.outside.yaml` の components。

## 進め方

main へ直に commit して push する。CI・gate は無い。確かめは commit の前に手元で回す。
版は 部品ごとの git の tag で切る。
