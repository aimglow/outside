# 03. テストの分類 — test の規則の正本への対応

**test の書き方の規則の正本は ccark の `docs/conventions/test-writing.md`（全 project 共通、R0〜R7・V1〜V3、各 seat の起動時の注入で届く）である。** この章は 規則を自前で持たず、分類 A / B / C を その規則番号へ対応させるだけにする（規則集を 2 つにしない）。

目的は 2 つ: ①test の結果を 入力だけで決まる形にし cache を効かせて 同じ入力の test を二度回さない（build の slot の無駄な詰まりを減らす）②壊れた code で必ず落ちる test だけを残して 品質を担保する。`outside` と `libs/platform` の作り（外に触れる物を閉じ込める）は 規則 R2・R2b・R4 を 構造で満たすための物である。

---

## 1. 分類と規則の対応

| 分類 | 何を確かめるか | 守る規則 (test-writing.md) | 手段 |
| :--- | :--- | :--- | :--- |
| **A** ロジック・振る舞い | 入力に対して 処理・状態の移り変わりが要件どおりか | R0（要件から書く）・R1（結果は source だけで決まる）・R2（その時々の値を読まない）・V1（壊れた code で落ちる） | `outside` と `libs/platform` の mock を渡す |
| **B** 境界の検証・設定の組み立て | 必須の設定が欠けた時に誤りになるか・段階ごとに正しい設定が組み上がるか | R2 型 (a)（機体の env を読まない。設定は値で渡す）・V1 | mock の Env に 値の表を渡す |
| **C** 書いてはならない test | — | R0（文書や実環境の値を確かめる）・R2（実の env・時刻を読む）・R2b（mock で確かめられる物を実物で確かめる）・V1 違反（mock の答えをそのまま確かめる = 壊れた code でも落ちない）・FW や標準 library 自体の仕様の確かめ | 書かない。在れば消す |

検査 = `ccark testguard check --rules writing`（各 project の `.ccark-prepush.toml` の段 `testguard-writing`）。外を直に呼ぶ code の検査 = `outside` の lint-go・lint-ts・lint-dart。

---

## 2. 「設定の一致確認」に対する明確な判断基準

実運用で最も迷いやすい「環境変数や設定値に関するテスト」の判断基準を以下に固定します。

```text
[問い] ある設定値（例: API_URL, COOKIE_SECURE, TIMEOUT_MS）をテストしたい
  │
  ├─ Q1: その値は「実行環境（dev / stg / prod）ごとに変わる値」か？
  │    ├─ YES ──▶ 「環境によって値がセットされているか」をテストコードで見てはならない。
  │    │           必要なのは「未設定時に起動エラーになるか」「不正値でバリデーションが落ちるか」
  │    │           を仮想入力で検証する【分類 B（境界バリデーションテスト）】である。
  │    │
  │    └─ NO（全環境で必ず同じ値でなければならない不変の仕様）
  │         │
  │         ├─ そもそも環境変数にする必要はない。コード内の定数（const）にせよ。
  │         └─ 条件に応じて動的に組み立てる仕様（例: prodならSecure=true）であれば、
  │            仮想入力を渡して出力オブジェクトを検証する【分類 B（設定仕様テスト）】を書け。
```

### 具体例で見る「良いテスト（分類 B）」と「愚かなテスト（分類 C）」

- **NG（分類 C — 書いてはいけない）:**
  ```typescript
  // 実機の process.env や実 .env ファイルを読み込んでアサートしている
  // → 実行環境が変わると落ちる、または環境依存でキャッシュが壊れる
  test('環境変数が設定されていること', () => {
    expect(process.env.SESSION_SECRET).toBeDefined();
    expect(process.env.COOKIE_SAMESITE).toBe('Strict');
  });
  ```
- **OK（分類 B — 書くべき仕様テスト）:**
  ```typescript
  // 仮想環境変数マップを注入し、「設定組み立てロジックの仕様」を検証している
  // → どのマシンでいつ実行しても 100% 同じ結果になり、キャッシュが効く
  test('production指定時はSecureかつStrictなCookieオプションが生成されること', () => {
    const out = createMockOutside();
    out.setEnv('APP_STAGE', 'production');

    const opts = buildSessionCookieOptions(out);

    expect(opts).toEqual({
      httpOnly: true,
      secure: true,
      sameSite: 'strict',
      path: '/',
    });
  });
  ```

---

## 3. 分類 C の型（規則番号は 1. の表）

Claude Code は以下のテストを自動生成した場合、自己レビュー段階で必ず削除しなければなりません。

1. **実環境依存アサーション（Environment-Coupled Test）**
   - 実機の `process.env`、実ファイルシステム上の `/Users/...`、実行日の `new Date()` を期待値や入力値に使うテスト。
2. **モックのオウム返しテスト（Tautological Mock Test）**
   - `mock.exec` に `"ok"` を登録し、単に `mock.exec` を呼ぶだけのラッパーを呼んで `"ok"` が返ることを検証するテスト（変換ロジックや分岐ロジックを一切通っていない）。
3. **フレームワーク・標準ライブラリ自体のテスト**
   - Next.js のルーター仕様、Angular の DI コンテナの基本動作、Flutter 標準 `Text` Widget の描画、Go 標準 `json.Marshal` の仕様そのものを検証するテスト。
4. **Unit テストへの実外部接続の混入**
   - Unit テストファイル内で実 DB、実ソケット、実 HTTP、実 CLI コマンドを叩くテスト。

---

## 4. 実外部との結合テストの置き場（outside / adapters の中だけ）

実 OS・実ツール・実ネットワークとの結合を確かめるテストは、**外部依存を吸収する `outside` と `adapters` の中にだけ置く**。アプリ（`apps/*`）とアプリの業務ロジックには、実外部に触れるテストを一切置かない（tag で隔離して置く形も取らない）。

| 置き場 | 実外部に触れるテスト | 普段回す Unit テスト |
| :--- | :--- | :--- |
| **`outside`（別リポジトリ）** | `Real` の自己検証テスト（自分で作った一時ディレクトリ・自分で起こした子プロセスだけを触る）。`outside` のリポジトリの中だけで回し、本体プロジェクトでは回さない。 | `Mock` の決定論テストと、`Real` / `Mock` の契約テスト |
| **`adapters/<feature>`** | `real` の結合テスト。各ランタイムの隔離形式で分離する（Go: `//go:build integration`、TS: `*.integration.spec.ts`、Dart: `@Tags(['integration'])`）。 | `mock` を使う契約テスト |
| **`apps/*`** | **置かない** | `outside` と `adapters` の `Mock` だけを注入した分類 A / B |

> **AI への鉄則:** アプリの機能開発では実外部に触れるテストを書かない。実外部との結合が要るなら、その境界を `adapters` に切り出し、結合テストは `adapters` の中に書く。
