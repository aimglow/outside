# AI自律開発のための決定的テスト基盤 設計・製造規約ドキュメント群

本ドキュメント群は、**Nx モノレポ構成**を土台とし、`Go` / `Dart (Flutter)` / `TypeScript (Next.js, Angular)` のマルチランタイム環境において、**すべての単体テスト（Unit Test）を 100% 決定的（Deterministic）かつ環境非依存（Hermetic）に保ち、テストキャッシュを最大限に効かせるためのアーキテクチャおよび AI（Claude Code）向け製造規約**を体系化したものです。

---

## 1. 解決する課題

AI エージェントによる自律開発では、プロンプトによる自然言語指示だけではコンテキスト伸長とともにルールが忘却され、以下の問題が発生します。

1. **非決定要素の混入によるキャッシュ破壊:**
   現在時刻、実ファイルシステム、PID、動的ポート、環境変数、実機絶対パス、Go の `map` 順序などがテストに混入し、機体や実行時刻が変わるたびにテスト結果が変動してキャッシュが外れる。
2. **CI / テスト実行時間の肥大化:**
   外部コマンドや実IOを伴うテストが毎回実行され、軽微な修正でも 1〜2 時間の待ち時間・手戻りが発生する。
3. **「愚かなテスト」の増殖:**
   環境変数の一致確認だけのテストやモックのオウム返しテストが量産され、本来のビジネスロジックの品質保証が機能しない。

---

## 2. 解決アーキテクチャの全体像

Nx ワークスペース内の `libs/outside/`（OS・ランタイム境界）と `libs/platform/`（adapters 層 = 外部の依存先ごとに 1 lib）の 2 層で、外部境界を完全に隔離します。

```text
workspace/
├── nx.json
├── CLAUDE.md                              # ワークスペース全体のAI製造規約（07を配置）
├── apps/
│   ├── ai-orchestrator/                   # [Go] AI開発基盤（Session Relay / 台帳 / 監視Daemon）
│   ├── web-portal/                        # [Next.js] 管理画面・SSRアプリ
│   ├── admin-console/                     # [Angular] 業務SPAアプリ
│   └── mobile-client/                     # [Flutter] モバイルアプリ
├── libs/
│   ├── outside/                           # 【第1層】OS・言語ランタイム境界（共有・不変・タグ同期）
│   │   ├── go/                            # afero.Fs, Clock(Advance付), CmdRunner, IdGen, SysInfo
│   │   ├── next/                          # node:fs + memfs, exec, env, now, idGen
│   │   ├── angular/                       # Browser Storage(Mock), now, idGen, location/navigator
│   │   └── flutter/                       # package:file, package:clock, ProcessRunner, Env, IdGen
│   └── platform/                          # 【第2層】adapters 層 = 外部の依存先ごとに 1 lib（interface / real / mock）
│       ├── pg-postgres-go/                # 例: PostgreSQL への接続境界
│       ├── authn-google-iap-go/           # 例: Google IAP の認証境界
│       ├── git-cli-go/                    # 例: git CLI の境界
│       └── auth-cookie-next/              # 例: Next.js の Cookie 境界
└── scripts/
    ├── sync-outside.sh                    # outside 別リポジトリから git archive でタグ同期
    └── check-forbidden.sh                 # 禁止構文・実機絶対パス検出スクリプト
```

---

## 3. ドキュメント一覧と読み方

| ファイル名 | 対象読者 | 内容の概要 |
| :--- | :--- | :--- |
| **`01_architecture_overview.md`** | アーキテクト / 開発者 | Nx ワークスペース構成、`outside/{go,angular,next,flutter}` と `adapters` の責務境界、別リポジトリ＋`git archive` 運用の整合性 |
| **`02_non_deterministic_catalog.md`** | 全開発者 / AI | キャッシュを破壊する全非決定要素カタログ（AI開発基盤特有の PID・UDS・FS監視・Ticker・Map順序から BtoC 端末境界まで） |
| **`03_test_classification_and_anti_patterns.md`** | 全開発者 / AI | テストの目的と手段の分類（分類A/B/C）、愚かなテストの排除基準、Integration テストの物理的隔離ルール |
| **`04_outside_spec_and_implementations.md`** | 基盤実装者 / AI | `libs/outside/{go,angular,next,flutter}` の標準インターフェースと `Real` / `Mock` 完全実装リファレンス |
| **`05_adapters_design_and_patterns.md`** | アプリ実装者 / AI | `libs/platform/` および `apps/*/platform/` の 3 ファイル分離（`interface` / `real` / `mock`）と具体実装パターン |
| **`06_lint_and_nx_enforcement.md`** | Tech Lead / CI担当 | `@nx/enforce-module-boundaries`、`golangci-lint`、ESLint、Dart禁則チェック、`nx.json` キャッシュ最適化設定 |
| **`07_agents_md_app_template.md`** | **Claude Code（本体用）** | 各アプリ・本体ワークスペースにそのまま配置する `CLAUDE.md` 完成版（判断基準・雛形・Relayプロトコル完備） |
| **`08_agents_md_outside_gatekeeper.md`** | **Claude Code（門番用）** | `outside` 基盤専用セッションに読み込ませる厳格な門番 `CLAUDE.md`（業務ロジック・FW依存混入防止） |
| **`09_operations_and_ci_scripts.md`** | Tech Lead / AI | 専用セッションへの `relay` 運用手順、`release.sh`（マイクロCI/CD）、`sync-outside.sh`（クリーン同期）スクリプト一式 |

---

## 4. 導入ロードマップ（推奨ステップ）

1. **Step 1（基盤の確定）:** `04` および `08` を基に `outside` 基盤を整備し、`release.sh` で初回タグ（`v0.1.0`）を発行、`sync-outside.sh` で `libs/outside/{go,angular,next,flutter}` へ展開する。
2. **Step 2（機械的ガードレールの設置）:** `06` の Lint ルール（生IO・生時刻・実機絶対パスの禁止）と Nx モジュール境界ルールをワークスペースに導入する。
3. **Step 3（AI製造規約の配置）:** `07` をワークスペースルートの `CLAUDE.md` として配置し、新規実装およびリファクタリングから全適用する。
4. **Step 4（最優先領域の移行）:** 最もキャッシュ効果の高い **AI開発基盤（セッションRelay・台帳・監視Daemon）** から `outside` / `adapters` へ移行し、CI 実行時間を数時間から数分へ短縮する。
5. **Step 5（業務システム・BtoCアプリへの展開）:** `libs/platform` に共通アダプタ（Cookie、認証、HTTP等）を整備し、Next.js / Angular / Flutter アプリへ横展開する。
