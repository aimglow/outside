// fake の振る舞い。渡した値だけで決まり 呼ばれた内容が読める事を確かめる。

import { describe, expect, it } from 'vitest';
import {
  createFakeClock,
  createFakeConfig,
  createFakeHttp,
  createFakeSet,
  createFakeStorage,
} from '../src/fake/index';

const START = Date.UTC(2026, 0, 2, 3, 4, 5);

describe('TC-OUT-CLOCK-FAKE fake の Clock', () => {
  it('決めた時刻を返し advance した分だけ進む', () => {
    const clock = createFakeClock(START);
    expect(clock.now()).toBe(START);
    expect(clock.now()).toBe(START);
    clock.advance(1500);
    expect(clock.now()).toBe(START + 1500);
    clock.advance(0);
    expect(clock.now()).toBe(START + 1500);
    expect(new Date(clock.now()).toISOString()).toBe('2026-01-02T03:04:06.500Z');
  });

  it('timer は 期限に届いた advance で呼ばれる。実時間を待たない', () => {
    const clock = createFakeClock(START);
    const firedAt: number[] = [];
    clock.setTimeout(() => firedAt.push(clock.now()), 60 * 60 * 1000);
    clock.advance(60 * 60 * 1000 - 1);
    expect(firedAt).toEqual([]);
    clock.advance(1);
    expect(firedAt).toEqual([START + 60 * 60 * 1000]);
    expect(clock.pendingTimers()).toBe(0);
  });

  it('1 回の advance の中で 期限の順に呼ばれ callback が見る時刻は その期限', () => {
    const clock = createFakeClock(START);
    const log: Array<[string, number]> = [];
    clock.setTimeout(() => log.push(['late', clock.now()]), 300);
    clock.setTimeout(() => log.push(['early', clock.now()]), 100);
    clock.setTimeout(() => log.push(['beyond', clock.now()]), 1001);
    clock.advance(1000);
    expect(log).toEqual([
      ['early', START + 100],
      ['late', START + 300],
    ]);
    expect(clock.now()).toBe(START + 1000);
    expect(clock.pendingTimers()).toBe(1);
  });

  it('callback の中で掛けた timer も 期限が届いていれば 同じ advance で呼ばれる', () => {
    const clock = createFakeClock(START);
    const firedAt: number[] = [];
    const tick = (): void => {
      firedAt.push(clock.now() - START);
      if (firedAt.length < 3) clock.setTimeout(tick, 100);
    };
    clock.setTimeout(tick, 100);
    clock.advance(250);
    expect(firedAt).toEqual([100, 200]);
    clock.advance(50);
    expect(firedAt).toEqual([100, 200, 300]);
  });

  it('clearTimeout した timer は advance しても呼ばれない', () => {
    const clock = createFakeClock(START);
    let fired = false;
    const handle = clock.setTimeout(() => {
      fired = true;
    }, 10);
    clock.clearTimeout(handle);
    clock.advance(100);
    expect(fired).toBe(false);
  });

  it('負の advance は 誤り', () => {
    const clock = createFakeClock(START);
    expect(() => clock.advance(-1)).toThrow('non-negative');
    expect(() => clock.advance(Number.NaN)).toThrow('non-negative');
  });

  it('呼ばれた内容が読める', () => {
    const clock = createFakeClock(START);
    clock.now();
    const handle = clock.setTimeout(() => {}, 250);
    clock.clearTimeout(handle);
    expect(clock.calls).toEqual([
      { method: 'now' },
      { method: 'setTimeout', ms: 250 },
      { method: 'clearTimeout' },
    ]);
  });
});

describe('REQ-OUT-FAKE fake の Http', () => {
  it('決めた応答を返し 送られた内容を 送られた順に記録する', async () => {
    const http = createFakeHttp([
      {
        method: 'POST',
        url: 'https://api.example.test/orders',
        response: { status: 201, headers: { Location: '/orders/7' }, body: '{"id":7}' },
      },
      { method: 'GET', url: 'https://api.example.test/orders/7', response: { status: 200 } },
    ]);
    const created = await http.request({
      method: 'POST',
      url: 'https://api.example.test/orders',
      headers: { 'content-type': 'application/json' },
      body: '{"item":"a"}',
    });
    const fetched = await http.request({ method: 'GET', url: 'https://api.example.test/orders/7' });
    expect(created).toEqual({ status: 201, headers: { location: '/orders/7' }, body: '{"id":7}' });
    expect(fetched).toEqual({ status: 200, headers: {}, body: '' });
    expect(http.calls).toEqual([
      {
        method: 'POST',
        url: 'https://api.example.test/orders',
        headers: { 'content-type': 'application/json' },
        body: '{"item":"a"}',
      },
      { method: 'GET', url: 'https://api.example.test/orders/7' },
    ]);
  });

  it('決めていない request は 何が無いかを名乗って reject し 記録には残る', async () => {
    const http = createFakeHttp();
    await expect(
      http.request({ method: 'GET', url: 'https://api.example.test/unknown' }),
    ).rejects.toThrow('no response configured for GET https://api.example.test/unknown');
    expect(http.calls).toHaveLength(1);
  });
});

describe('REQ-OUT-FAKE fake の Storage', () => {
  it('空から始まり 別に作った物と 中身を分け合わない', () => {
    const first = createFakeStorage();
    first.set('token', 'abc');
    const second = createFakeStorage();
    expect(second.get('token')).toBeNull();
    expect(first.get('token')).toBe('abc');
  });

  it('呼ばれた内容が読める', () => {
    const storage = createFakeStorage();
    storage.set('k', 'v');
    storage.get('k');
    storage.remove('k');
    expect(storage.calls).toEqual([
      { method: 'set', key: 'k', value: 'v' },
      { method: 'get', key: 'k' },
      { method: 'remove', key: 'k' },
    ]);
  });
});

describe('REQ-OUT-FAKE fake の Config', () => {
  it('渡した表の値だけを返し 機体の環境変数は 結果に現れない', () => {
    const config = createFakeConfig({ API_URL: 'https://api.example.test' });
    expect(config.get('API_URL')).toBe('https://api.example.test');
    // 多くの機体に在る名前でも 表に無ければ 無い。
    for (const name of ['PATH', 'HOME', 'USER', 'TZ', 'NODE_ENV']) {
      expect(config.get(name)).toBeUndefined();
    }
  });

  it('渡した表を後から書き換えても 結果は変わらない', () => {
    const table: Record<string, string> = { A: '1' };
    const config = createFakeConfig(table);
    table['A'] = '2';
    expect(config.get('A')).toBe('1');
  });

  it('引かれた名前が読める', () => {
    const config = createFakeConfig({ A: '1' });
    config.get('A');
    config.get('B');
    expect(config.calls).toEqual(['A', 'B']);
  });
});

describe('REQ-OUT-FAKE fake の set', () => {
  it('渡した値から 4 つの fake を作る', async () => {
    const set = createFakeSet({
      startMs: START,
      routes: [{ method: 'GET', url: 'https://api.example.test/', response: { status: 204 } }],
      config: { MODE: 'test' },
    });
    expect(set.clock.now()).toBe(START);
    expect((await set.http.request({ method: 'GET', url: 'https://api.example.test/' })).status).toBe(204);
    expect(set.storage.get('k')).toBeNull();
    expect(set.config.get('MODE')).toBe('test');
  });
});
