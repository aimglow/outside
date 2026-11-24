# 05. `adapters` 設計・Real/Mock 分離と実装パターン

`libs/platform/` および `apps/<app>/src/platform/` は、フレームワーク固有 API（Cookie、Flutter プラグイン等）やプロジェクト固有の外部境界（セッション間 IPC 通信、台帳ストレージ、外部 API 等）を隔離するレイヤーです。

---

## 0. adapters 層 (`libs/platform/`) の命名規則（厳格・機械で落とす）

`libs/platform/` の直下の dir は 次の形だけを許す。形に合わない名前は lint が赤にする。

```text
<機能>-<依存先>-<runtime>
```

| 部分 | 許す値 |
| :--- | :--- |
| `<機能>` | 全 project 共通の語の正本 = `outside` の `lint-platform/naming/rules.go`。project 固有の語は repo の根の `outside-platform.toml` の `capabilities` に宣言する（その project の review で決める）。 |
| `<依存先>` | 外部の製品・プロトコルの名前（小文字・`-` 区切り。例: `postgres`・`google-iap`・`cloudflare-access`・`git`・`podman`・`atlas`・`buf`・`ssh`・`shared-preferences`）。 |
| `<runtime>` | `go`・`next`・`angular`・`flutter` のどれか（`outside` の部品と同じ 4 つ）。Node で動く TypeScript は Next.js でなくても `next`（Node だけで動く lib も `next`）。 |

- adapters 層に置くのは real が外（通信・file・子 process・機体の値）を触る lib だけ。外を触らない部品（形式の変換・計算・集計など 値を受けて値を返すだけの物）は real と mock に分ける意味が無いので adapters ではなく 業務の側の lib に置く（mock を作らず そのまま test する）。外を触る部分と触らない部分を持つ物は 触る部分だけを adapters の lib に切り出す。
- lib を作るのは 依存先に固有の取り決め（出力や答えの形・終了コードの意味・protocol・認証・状態）を読み書きする時だけ。コマンドを起こして終了コードを見るだけなら `outside` の子 process の口（Go = `Proc`）を直に使い lib を作らない（test は Proc の mock を渡す）。同じ取り決めを 2 か所以上が読み解いていたら 1 つの lib にまとめる。
- 1 lib = 1 依存先（機械で判定できない = lib を足す変更の 設計の review で見る）。機能に `common`・`util`・`shared`・`helper` 等の 中身を表さない語を使わない（検査が拒む）。
- 依存先の実物を起こす test の部品（例: 決めた版の DB を起こす部品）は その依存先の lib の 結合テストの中に置き、独立の lib にしない。
- lib は `lint-platform` の `scaffold` で作る。検査 = 同じ道具の `check`（enforcement §4.1）。
- 表せない物が出た時: 機能の語が無い → `outside-platform.toml` に足す / 3 file に分けられない依存先 → project の中で例外を作らず outside 門番へ依頼する。

## 1. 最重要ルール：3 ファイル分離による権限隔離

AI に `adapters/` の作成を許可すると、手抜きをして **`Mock` 実装の中に `Date.now()` や実機パス（`/Users/...`）をハードコードするリスク**が発生します。

これを防ぐため、すべてのアダプタは必ず以下の **3 ファイル構造** に分割し、`real` だけに生の外部 API 利用を許可します。

```text
libs/platform/authn-cookie-next/
├── interface.ts   # 純粋な型・インターフェース定義のみ（依存ゼロ）
├── real.ts        # 【唯一許可】Next.js cookies() やブラウザ API を呼ぶ薄い接着剤
└── mock.ts        # 【厳格制限】インメモリ実装。ビジネスロジックと同等のLint制限を受ける
```

Go / Dart の場合のファイル命名規則：
- **Go:** `interface.go` / `adapter_real.go` / `adapter_mock.go`
- **Dart:** `interface.dart` / `adapter_real.dart` / `adapter_mock.dart`

### `Mock` 実装の鉄則
- `Mock` 自身が時刻や ID を生成する必要がある場合は、`Mock` 内で `Date.now()` や `time.Now()` を絶対に呼ばず、**コンストラクタで `MockOutside` を受け取るか、テストから固定値を渡す**こと。
- 仮想ファイルパスを扱う場合は、必ず `/virtual/` をルートとした論理パス（例: `"/virtual/ipc/session.sock"`）のみを使用すること。

---

## 2. パターン①：Next.js Cookie アダプタ（`libs/platform/authn-cookie-next`）

プロジェクトやステージごとに属性や運用が変わる Cookie は `outside` ではなく `adapters` で隔離します。

```typescript
// libs/platform/authn-cookie-next/interface.ts
export interface CookieOptions {
  httpOnly: boolean;
  secure: boolean;
  sameSite: 'strict' | 'lax' | 'none';
  maxAgeSec: number;
  path: string;
}

export interface AuthCookieAdapter {
  getSessionToken(): Promise<string | undefined>;
  setSessionToken(token: string, options: CookieOptions): Promise<void>;
  clearSessionToken(): Promise<void>;
}
```

```typescript
// libs/platform/authn-cookie-next/real.ts
import { cookies } from 'next/headers';
import type { AuthCookieAdapter, CookieOptions } from './interface';

const COOKIE_NAME = '__Host-session';

export class NextAuthCookieAdapter implements AuthCookieAdapter {
  async getSessionToken(): Promise<string | undefined> {
    const store = await cookies();
    return store.get(COOKIE_NAME)?.value;
  }

  async setSessionToken(token: string, options: CookieOptions): Promise<void> {
    const store = await cookies();
    store.set(COOKIE_NAME, token, {
      httpOnly: options.httpOnly,
      secure: options.secure,
      sameSite: options.sameSite,
      maxAge: options.maxAgeSec,
      path: options.path,
    });
  }

  async clearSessionToken(): Promise<void> {
    const store = await cookies();
    store.delete(COOKIE_NAME);
  }
}
```

```typescript
// libs/platform/authn-cookie-next/mock.ts
import type { AuthCookieAdapter, CookieOptions } from './interface';

export class MockAuthCookieAdapter implements AuthCookieAdapter {
  public token: string | undefined = undefined;
  public lastOptions: CookieOptions | undefined = undefined;

  async getSessionToken(): Promise<string | undefined> {
    return this.token;
  }

  async setSessionToken(token: string, options: CookieOptions): Promise<void> {
    this.token = token;
    this.lastOptions = options;
  }

  async clearSessionToken(): Promise<void> {
    this.token = undefined;
    this.lastOptions = undefined;
  }
}
```

---

## 3. パターン②：AI開発基盤 セッション間 IPC アダプタ（Go）

Unix Domain Socket（UDS）やローカル TCP を使ったセッション同士の `relay` 通信を、実ソケットを開かずにテスト可能にするパターンです。

```go
// libs/platform/session-ipc/interface.go
package sessionipc

import "context"

type Message struct {
	FromSession string
	ToSession   string
	Payload     string
}

type Transport interface {
	Send(ctx context.Context, msg Message) error
	Receive(ctx context.Context, sessionID string) (Message, error)
}
```

```go
// libs/platform/session-ipc/adapter_mock.go
package sessionipc

import (
	"context"
	"fmt"
	"sync"
)

// MockTransport は実ソケットを開かず、メモリ上のキューで決定論的にメッセージを受け渡す。
type MockTransport struct {
	mu     sync.Mutex
	queues map[string][]Message
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		queues: make(map[string][]Message),
	}
}

func (f *MockTransport) Send(_ context.Context, msg Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queues[msg.ToSession] = append(f.queues[msg.ToSession], msg)
	return nil
}

func (f *MockTransport) Receive(_ context.Context, sessionID string) (Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	q := f.queues[sessionID]
	if len(q) == 0 {
		return Message{}, fmt.Errorf("no message for session: %s", sessionID)
	}
	msg := q[0]
	f.queues[sessionID] = q[1:]
	return msg, nil
}
```

---

## 4. パターン③：Angular DI 連携（`InjectionToken`）

Angular では `BrowserOutside` と各種アダプタを `InjectionToken` 経由で注入し、本番（`app.config.ts`）とテスト（`TestBed`）で差し替えます。

```typescript
// apps/admin-console/src/app/core/tokens.ts
import { InjectionToken } from '@angular/core';
import type { BrowserOutside } from '@outside/angular';

export const BROWSER_OUTSIDE = new InjectionToken<BrowserOutside>('BROWSER_OUTSIDE');
```

```typescript
// apps/admin-console/src/app/features/auth/token.service.spec.ts
import { TestBed } from '@angular/core/testing';
import { createMockBrowserOutside } from '@outside/angular';
import { BROWSER_OUTSIDE } from '../../core/tokens';
import { TokenService } from './token.service';

describe('TokenService', () => {
  it('期限切れトークンを固定時刻で正しく判定できる', () => {
    const mockOut = createMockBrowserOutside('2026-06-01T00:00:00.000Z');
    mockOut.localStorage.setItem('expires_at', '2026-05-31T23:59:59.000Z');

    TestBed.configureTestingModule({
      providers: [
        TokenService,
        { provide: BROWSER_OUTSIDE, useValue: mockOut },
      ],
    });

    const service = TestBed.inject(TokenService);
    expect(service.isTokenValid()).toBe(false);
  });
});
```

---

## 5. パターン④：Flutter プラグイン隔離（`SharedPreferences` 等）

Flutter のネイティブプラグインは共有 `outside/flutter` に入れず、アプリまたは共有 `adapters` に切り出します。

```dart
// libs/platform/kv-storage/interface.dart
abstract class KvStorage {
  Future<String?> read(String key);
  Future<void> write(String key, String value);
}

// libs/platform/kv-storage/adapter_mock.dart
class MockKvStorage implements KvStorage {
  final Map<String, String> _data = {};

  @override
  Future<String?> read(String key) async => _data[key];

  @override
  Future<void> write(String key, String value) async {
    _data[key] = value;
  }
}
```
