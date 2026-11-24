# CLAUDE.md — Nx ワークスペース テスト決定性・外部依存隔離 製造規約

> 本ファイルは、本体ワークスペース（各アプリおよび `libs/platform`）を担当する Claude Code（CC）が常時遵守すべき製造規約です。

---

## 0. 最上位目的（Absolute Mandate）

本ワークスペースにおけるすべての単体テスト（Unit Test）は、**100% 決定的（Deterministic）かつ環境非依存（Hermetic）** でなければならない。
- 実行するマシン・OS・タイムゾーン・現在日時・ディレクトリパス・環境変数・PID・ネットワーク状態が変わっても、テストの入力と出力は **1 ビットも変化してはならない**。
- Nx のテストキャッシュを無効化させるコード（生 IO・現在時刻・未固定乱数・実環境変数・実機絶対パス・非決定的な Map 順序）の混入を一切禁ずる。

---

## 1. test の書き方（規則の正本 = ccark の `docs/conventions/test-writing.md`）

test を書く前に ccark の `docs/conventions/test-writing.md`（R0〜R7・V1〜V3 と 型 (a)〜(j)）を読み、それに従う。分類 A / B / C と規則番号の対応 = `reference.deterministic-testing.test-classification.md` §1。
- 書くのは A（ロジック・振る舞い）と B（境界の検証・設定の組み立て）だけ。どちらも `outside` と `libs/platform` の mock を渡し、壊れた code で必ず落ちること（V1）。
- C（実環境の値の確かめ・mock の答えの確かめ・FW 自体の確かめ・app の中で実物に触れる test）は書かない。在れば消す。
- 実物に触れる test は `libs/platform/<lib>` の結合 test（tag `integration`）だけに置く。

## 2. 外部境界の適用範囲と配置先ルール

以下の領域はすべて非決定的な外部境界である。ビジネスロジックおよび Unit テスト内での直接呼び出しを禁止し、必ず `outside` または `adapters` 経由でアクセスすること。

| 領域 | 禁止される直接呼び出し | 正しい隔離先 |
| :--- | :--- | :--- |
| **時刻・タイマー** | `time.Now`, `time.Sleep`, `new Date()`, `Date.now()`, `DateTime.now()` | `libs/outside/{go,next,angular,flutter}` の `Clock` / `now` |
| **ファイルシステム** | `os.*`, `node:fs`, `dart:io`, `Directory.current`, `os.TempDir` | `libs/outside/{go,next,flutter}` の `Fs`（仮想パス `/virtual/...` のみ使用） |
| **環境変数** | `os.Getenv`, `process.env`, `Platform.environment` | `libs/outside/{go,next,flutter}` の `Getenv` / `env` |
| **乱数・ID 生成** | `math/rand`, `crypto/rand`, `Math.random()`, `randomUUID()`, `Random()` | `libs/outside/{go,next,angular,flutter}` の `Id` / `newId` |
| **外部プロセス実行** | `exec.Command`, `child_process`, `Process.run` | `libs/outside/{go,next,flutter}` の `Cmd` / `exec` / `process` |
| **マシン・プロセス情報** | `os.Getpid`, `os.Hostname`, `os.UserHomeDir`, `process.cwd`, `__dirname` | `libs/outside/go` の `Sys`（他言語では仮想固定値を使用） |
| **ブラウザ Storage** | `localStorage`, `sessionStorage` | `libs/outside/angular` の `localStorage` / `sessionStorage` |
| **Cookie / FW API / IPC** | `next/headers` (`cookies()`), `SharedPreferences`, 実 UDS ソケット, 外部 SDK | `libs/platform/*` または `apps/<app>/src/platform/*` |

---

## 3. `adapters/` 実装および仮想パス・ソート規約

1. **3 ファイル厳格分離:**
   - 新規アダプタを作成する場合は、必ず `interface`・`real`（`*_real.go` / `*_real.dart`）・`mock`（`*_mock.go` / `*_mock.dart`）の 3 ファイルに分割すること。
   - 生の外部 API・FW API の呼び出しは `real` ファイル内でのみ許可される。**`mock` ファイル内で現在時刻・乱数・実ファイル・実機パス取得関数を呼ぶことを固く禁ずる。**
2. **仮想パスの統一（実機絶対パスの禁止）:**
   - テストおよび `Mock` 実装内のパスは、すべて `/virtual/` をルートとした論理パス（例: `"/virtual/config/app.json"`）を使用すること。`/Users/...`, `/home/...`, `C:\...` 等のハードコードを禁ずる。
3. **決定論的ソート規約（Go 必須）:**
   - Go で台帳データ・キャッシュキー・JSON 出力を生成する際、`map` を直接シリアライズまたは反復してハッシュ化してはならない。必ずスライスに変換し `slices.Sort` / `slices.SortFunc` で決定論的にソートしてから出力すること。

---

## 4. `outside` 基盤改修プロトコル（専用セッションへの Relay 必須）

`libs/outside/<部品>/`（go・next・angular・flutter・lint-go・lint-ts・lint-dart・lint-platform）は `outside` repo の部品の tag から `git archive` で置いた写しである（`docs/deterministic-testing/` も同じ）。
**本ワークスペースのセッションから `libs/outside/` 配下と `docs/deterministic-testing/` のファイルを直接編集することを固く禁ずる。**

### 機能が不足している場合の必須フロー
1. **責務の切り分け:**
   - Pure な OS・ランタイムプリミティブ（未実装の FS 操作、タイマー機能等）のみ `outside` の追加対象とする。
   - Cookie、特定 CLI ラッパー、UDS/IPC、FW プラグインは `adapters/` に実装せよ。
2. **`outside` 専用セッションへの Relay:**
   - 不足機能が OS プリミティブである場合、業務背景やアプリ名を一切含めず、以下の形式で `outside` 専用セッションへ `relay` を依頼せよ。
   ```text
   【outside 機能追加依頼】
   - 対象の部品: go | next | angular | flutter | lint-go | lint-ts | lint-dart | lint-platform | docs
   - 追加したいOSプリミティブ概要:
   - 求めるシグネチャ案:
   - Mock実装に求める振る舞い:
   - 完了条件: Real/Mock両方のテストをPASSさせ、./scripts/release.sh <部品> <段> で発行された tag（例: go/v0.7.0）を返却すること。
   ```
3. **同期と再開:**
   - 返却された tag（例: `go/v0.7.0`）を受け取ったら、`./scripts/sync-outside.sh <部品> <tag>` を repo の根で実行して置き場をクリーン置換し、作業を再開せよ（運用の正本 = `reference.deterministic-testing.operations.md`）。

---

## 5. 言語・フレームワーク別 実装およびテスト標準パターン

### 5.1 Go（`libs/outside/go`）
```go
func TestDaemonHeartbeat(t *testing.T) {
	env := outside.NewMock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	env.Mock.SetEnv("DAEMON_MODE", "active")

	d := NewDaemon(env)
	d.Tick()
	env.Mock.Clock.Advance(10 * time.Second)
	d.Tick()

	// env.Fs や env.Mock.Cmd.History を検証
}
```

### 5.2 TypeScript — Next.js（`libs/outside/next`）
```typescript
import { describe, it, expect } from 'vitest';
import { createMockOutside } from '@outside/next';
import { buildReport } from './buildReport';

describe('buildReport', () => {
  it('固定時刻と仮想FSでレポートを生成できる', async () => {
    const out = createMockOutside('2026-01-01T00:00:00.000Z');
    out.setEnv('STAGE', 'prod');
    await out.fs.mkdir('/virtual/in', { recursive: true });
    await out.fs.writeFile('/virtual/in/data.txt', 'hello');

    await buildReport(out, '/virtual/in/data.txt', '/virtual/out/res.txt');

    expect(await out.fs.readFile('/virtual/out/res.txt', 'utf-8')).toBe(
      '[2026-01-01T00:00:00.000Z][prod] hello',
    );
  });
});
```

### 5.3 TypeScript — Angular（`libs/outside/angular`）
```typescript
import { TestBed } from '@angular/core/testing';
import { createMockBrowserOutside } from '@outside/angular';
import { BROWSER_OUTSIDE } from '../../core/tokens';
import { PrefsService } from './prefs.service';

describe('PrefsService', () => {
  it('仮想localStorageへ設定を保存できる', () => {
    const mockOut = createMockBrowserOutside('2026-01-01T00:00:00.000Z');
    TestBed.configureTestingModule({
      providers: [
        PrefsService,
        { provide: BROWSER_OUTSIDE, useValue: mockOut },
      ],
    });

    const service = TestBed.inject(PrefsService);
    service.saveTheme('dark');
    expect(mockOut.localStorage.getItem('theme')).toBe('dark');
  });
});
```

### 5.4 Dart / Flutter（`libs/outside/flutter`）
```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:outside_flutter/outside.dart';
import 'package:mobile_client/features/draft/draft_repo.dart';

void main() {
  test('固定時刻で仮想FSに下書きを保存できる', () async {
    final out = Outside.mock(fixedTime: DateTime.utc(2026, 1, 1));
    final repo = DraftRepo(out);

    await repo.save('/virtual/drafts/1.txt', 'content');

    expect(
      await out.fs.file('/virtual/drafts/1.txt').readAsString(),
      '2026-01-01T00:00:00.000Z|content',
    );
  });
}
```

---

## 6. タスク完了前の必須自己検証ゲート（Verification Gate）

タスク完了を報告する前に、必ず Nx コマンドで以下を実行し、すべて PASS させること。

1. `ccark prepush-gate`（この検査の中で Nx の lint・test・境界と 禁止構文・実機パスの検査が回る。違反ゼロ・全 PASS であること）
3. 分類 C（愚かなテスト）に該当するテストを書いていないか自己点検し、該当する場合は削除すること。
