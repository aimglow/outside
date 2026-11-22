# outside

application とツールが 外 (時計・環境変数・file・git・子 process・作業場・通信・保存) を触る場所を
この部品 1 つに限るための 共通部品。全 project が同じ版を使う。

- 設計書: `docs/developer/design.outside.yaml`
- テスト仕様: `docs/developer/spec.outside.yaml`

## 置き場

| dir | 中身 |
|---|---|
| `go/` | Go の module (`github.com/aimglow/outside/go`、偽物 = `outsidefake`) |
| `ts/` | TypeScript の library (`@aimglow/outside`、偽物 = `@aimglow/outside/fake`) |
| `dart/` | Dart の package (`outside`) |
| `lint/` | stack ごとの lint の設定 (外を直に呼ぶ code を落とす) |

## 進め方

main へ直に commit して push する。CI・gate は無い。確かめは commit の前に手元で回す。
版は git の tag で切る。
