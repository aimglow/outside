# 01. アーキテクチャ設計と Nx モノレポ構成

## 1. 基本思想：外部境界の 2 層隔離

テストのキャッシュを 100% 効かせるための鉄則は、**「ビジネスロジック（テスト対象の 95%）から、OS・ランタイム・外部ネットワーク・フレームワーク固有の副作用を完全に追い出すこと」**です。

これを破綻なく運用するため、外部境界を **不変の OS・ランタイム層（`outside`）** と **変化しうるプロジェクト・FW 固有層（`adapters`）** の 2 層に明確に分離します。

| レイヤー | Nx 配置パス | 管理単位 | 責務（何を持つか） | 選定の絶対基準 |
| :--- | :--- | :--- | :--- | :--- |
| **第1層: `outside`** | `libs/share/outside/{go,angular,next,flutter}` | 別リポジトリ管理<br>＋ `git archive` タグ同期 | 各言語・ランタイムの純粋な OS / 環境境界の `Real` と `Fake` セット | **「3年後に全く別の新規プロジェクトを作るとき、1ミリも変えずに使えるか？」→ YES のみ** |
| **第2層: `adapters`** | `libs/share/adapters/*`<br>（または `apps/<app>/adapters/*`） | Nx モノレポ内で管理 | Cookie 運用、認証、特定 CLI ラッパー、UDS/IPC 通信、Flutter プラグイン等 | 特定のフレームワーク・外部サービス・業務仕様に少しでも依存するもの |

---

## 2. なぜ `outside` を `{go,angular,next,flutter}` の 4 区分にするのか

単なる言語単位（`go`, `ts`, `dart`）ではなく、**実行ランタイム環境**を基準に 4 サブディレクトリへ分割することで、過不足のない抽象化を実現します。

1. **`libs/share/outside/go`（システム・デーモン・CLI ランタイム）**
   - OS ファイルシステム（`afero`）、仮想時計＋Ticker 制御（`Clock`）、外部プロセス実行（`CmdRunner`）、環境変数（`Env`）、決定論的 ID 生成（`IdGen`）、マシン識別子（`SysInfo`）を提供。
2. **`libs/share/outside/next`（Node.js サーバー／SSR ランタイム）**
   - `node:fs/promises`（`memfs` 連携）、`child_process`（`exec`）、`process.env`（仮想環境変数マップ）、サーバー時刻（`now`）、決定論的 ID（`idGen`）を提供。
3. **`libs/share/outside/angular`（ブラウザ DOM／SPA ランタイム）**
   - ブラウザには `node:fs` が存在しない代わりに、`localStorage` / `sessionStorage` のインメモリ Fake、ブラウザ時計（`now`）、決定論的 ID（`idGen`）、`location` / `navigator` 情報の抽象インターフェースを提供。
4. **`libs/share/outside/flutter`（Dart / モバイル・デスクトップ ランタイム）**
   - `package:flutter` への依存を排除した Pure Dart 構成で、`package:file`（`MemoryFileSystem`）、`package:clock`（`Clock.fixed`）、`ProcessRunner`、`Env`、`IdGen` を提供。Flutter 固有の `MethodChannel` や `SharedPreferences` は `adapters/` 側で隔離する。

> **重要:** 4 区分に分かれていても、**「業務ドメイン用語・特定外部ツール名（`git`, `docker`, `ffmpeg` 等）・アプリ固有の設定キー」を `outside` に持ち込まない**という門番ルールは全 4 区分で共通です。

---

## 3. `libs/share/adapters` と `apps/<app>/adapters` の棲み分け

Nx モノレポ内のアダプタ配置は、再利用範囲によって 2 段階に分けます。

- **`libs/share/adapters/<feature>/`（複数アプリ共有アダプタ）**
  - 複数のアプリやサービスで同一仕様を共有する外部境界。
  - 例: `auth-cookie`（Next.js / Angular 共通の Cookie セキュリティ属性仕様）、`ledger-client`（機体間キャッシュ台帳へのアクセス境界）、`llm-gateway`（LLM API 呼び出し境界）。
- **`apps/<app>/src/adapters/<feature>/`（単一アプリ固有アダプタ）**
  - そのアプリケーションでしか使わない外部連携やフレームワーク境界。
  - 例: 特定アプリ専用の Flutter ネイティブプラグインラッパー、特定業務システムのレガシー SOAP/REST クライアント。

---

## 4. Nx ワークスペースにおける依存関係グラフ（DAG）

Nx の `@nx/enforce-module-boundaries` を用いて、以下の依存方向をコンパイル・Lint レベルで強制します。逆方向の依存は一切許可しません。

```text
[ apps/* ]  (ビジネスロジック / UI / エントリーポイント)
     │
     ├──────────────────────────────┐
     ▼                              ▼
[ libs/share/adapters/* ]    [ libs/share/outside/* ]
     │                              ▲
     └──────────────────────────────┘
```

- `outside` は `adapters` や `apps` に一切依存しない（**依存ゼロのリーフノード**）。
- そのため、`outside` のバージョンタグが変わらない限り、Nx の計算キャッシュ上で `outside` 自体の再テストは一切発生しません。
- `apps/*` のビジネスロジックは、テスト時に `outside` の `Fake` と `adapters` の `Fake` のみを注入して実行するため、**すべての Unit テストが外部環境から完全に切り離され、Nx のキャッシュヒット率が 100% になります**。

---

## 5. 別リポジトリ（`outside`）＋ `git archive` 運用との接続

`outside` を別リポジトリとして管理し、`git archive` で `libs/share/outside/{go,angular,next,flutter}` に展開する運用は、Nx モノレポと極めて高い親和性を持ちます。

### このハイブリッド運用が優れている理由
1. **認証・レジストリ問題のゼロ化:** プライベート npm / pub / Go proxy の認証設定が不要になり、ローカル・CI・コンテナのどこでも同じファイルハッシュで Nx キャッシュが効く。
2. **AI が実体を直接読める:** `libs/share/outside/` 配下に実ソースファイルが展開されているため、Claude Code が型定義や `Fake` のヘルパーメソッドを推測せずに直接 `Read` できる。
3. **本流への還元サイクルの確立:** 展開先（`libs/share/outside/`）の直接編集を禁止し、不足機能は必ず `outside` 専用セッションへ `relay` してタグ発行 → `sync-outside.sh` で取り込むフローにすることで、全プロジェクトで品質が統一される。
