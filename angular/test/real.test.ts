// real だけが持つ振る舞い (外へ渡す内容・外が無い時の誤り)。
// 外の物は test が渡すか test が global を差し替える。機体の物は読まない。

import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  createRealClock,
  createRealConfig,
  createRealHttp,
  createRealIdGen,
  createRealSet,
  createRealStorage,
  type FetchLike,
} from '../src/index';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('REQ-OUT-REAL-TESTED-HERE real の Http', () => {
  it('method・url・header・body を fetch へ そのまま渡す', async () => {
    const seen: Array<{ url: string; init: Parameters<FetchLike>[1] }> = [];
    const http = createRealHttp(async (url, init) => {
      seen.push({ url, init });
      return new Response('ok', { status: 200 });
    });
    await http.request({
      method: 'PUT',
      url: 'https://api.example.test/items/1',
      headers: { authorization: 'Bearer t' },
      body: '{"a":1}',
    });
    await http.request({ method: 'GET', url: 'https://api.example.test/items/1' });
    expect(seen).toEqual([
      {
        url: 'https://api.example.test/items/1',
        init: { method: 'PUT', headers: { authorization: 'Bearer t' }, body: '{"a":1}' },
      },
      { url: 'https://api.example.test/items/1', init: { method: 'GET' } },
    ]);
  });

  it('fetch を渡さなければ 使う時に global の fetch を取る', async () => {
    const http = createRealHttp();
    vi.stubGlobal('fetch', async () => new Response('from-global', { status: 202 }));
    const res = await http.request({ method: 'GET', url: 'https://api.example.test/' });
    expect(res.status).toBe(202);
    expect(res.body).toBe('from-global');
  });

  it('fetch が無い環境では 無い物を名乗って reject する', async () => {
    vi.stubGlobal('fetch', undefined);
    await expect(
      createRealHttp().request({ method: 'GET', url: 'https://api.example.test/' }),
    ).rejects.toThrow('fetch is not available');
  });
});

describe('REQ-OUT-REAL-TESTED-HERE real の Storage', () => {
  it('作るだけでは 保存を触らず 使う時に 無い物を名乗って throw する', () => {
    vi.stubGlobal('localStorage', undefined);
    vi.stubGlobal('sessionStorage', undefined);
    const local = createRealStorage();
    const session = createRealStorage('session');
    expect(() => local.get('k')).toThrow('localStorage is not available');
    expect(() => session.set('k', 'v')).toThrow('sessionStorage is not available');
  });

  it("'session' は global の sessionStorage を使う", () => {
    const table = new Map<string, string>();
    vi.stubGlobal('sessionStorage', {
      getItem: (key: string) => table.get(key) ?? null,
      setItem: (key: string, value: string) => void table.set(key, value),
      removeItem: (key: string) => void table.delete(key),
    });
    const storage = createRealStorage('session');
    storage.set('k', 'v');
    expect(table.get('k')).toBe('v');
    expect(storage.get('k')).toBe('v');
  });
});

describe('REQ-OUT-REAL-TESTED-HERE real の Config', () => {
  it('process.env に在る名前でも values に無ければ 無い (process を見ない)', () => {
    vi.stubGlobal('process', { env: { OUTSIDE_TEST_NAME: 'stubbed', A: 'from-env' } });
    const config = createRealConfig({ values: { A: 'from-values' } });
    expect(config.get('A')).toBe('from-values');
    expect(config.get('OUTSIDE_TEST_NAME')).toBeUndefined();
    expect(createRealConfig().get('OUTSIDE_TEST_NAME')).toBeUndefined();
  });

  it('process が無い環境では values だけを引く', () => {
    vi.stubGlobal('process', undefined);
    const config = createRealConfig({ values: { A: '1' } });
    expect(config.get('A')).toBe('1');
    expect(config.get('B')).toBeUndefined();
  });
});

describe('REQ-OUT-REAL-TESTED-HERE real の Clock', () => {
  it('timer が無い環境では 無い物を名乗って throw する', () => {
    const clock = createRealClock();
    vi.stubGlobal('setTimeout', undefined);
    expect(() => clock.setTimeout(() => {}, 0)).toThrow('setTimeout is not available');
  });
});

describe('REQ-OUT-REAL-TESTED-HERE real の IdGen', () => {
  const UUID_V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

  it('UUID v4 の形の ID を返す', () => {
    const idGen = createRealIdGen();
    for (let i = 0; i < 20; i += 1) expect(idGen.newId()).toMatch(UUID_V4);
  });

  it('使う時に global の crypto を取り randomUUID を crypto の method として呼ぶ', () => {
    const idGen = createRealIdGen();
    const stub = {
      label: 'stub',
      randomUUID(this: { label: string }) {
        return `${this.label}-uuid`;
      },
    };
    vi.stubGlobal('crypto', stub);
    expect(idGen.newId()).toBe('stub-uuid');
  });

  it('crypto か randomUUID が無い環境では 無い物を名乗って throw する', () => {
    const idGen = createRealIdGen();
    vi.stubGlobal('crypto', undefined);
    expect(() => idGen.newId()).toThrow('crypto is not available');
    vi.stubGlobal('crypto', {});
    expect(() => idGen.newId()).toThrow('crypto.randomUUID is not available');
  });
});

describe('REQ-OUT-INJECT real の set', () => {
  it('作るだけでは 外を触らない', () => {
    vi.stubGlobal('fetch', undefined);
    vi.stubGlobal('localStorage', undefined);
    vi.stubGlobal('process', undefined);
    vi.stubGlobal('crypto', undefined);
    const set = createRealSet();
    expect(Object.keys(set).sort()).toEqual(['clock', 'config', 'http', 'idGen', 'storage']);
  });
});
