// mock の振る舞い。渡した値だけで決まり 呼ばれた内容が読める事を確かめる。

import { describe, expect, it } from 'vitest';
import {
  createMockClock,
  createMockConfig,
  createMockHttp,
  createMockIdGen,
  createMockRandom,
  createMockSet,
  createMockStorage,
  DEFAULT_MOCK_RANDOM_SEED,
} from '../src/mock/index';

const START = Date.UTC(2026, 0, 2, 3, 4, 5);

describe('TC-OUT-CLOCK-MOCK mock の Clock', () => {
  it('決めた時刻を返し advance した分だけ進む', () => {
    const clock = createMockClock(START);
    expect(clock.now()).toBe(START);
    expect(clock.now()).toBe(START);
    clock.advance(1500);
    expect(clock.now()).toBe(START + 1500);
    clock.advance(0);
    expect(clock.now()).toBe(START + 1500);
    expect(new Date(clock.now()).toISOString()).toBe('2026-01-02T03:04:06.500Z');
  });

  it('timer は 期限に届いた advance で呼ばれる。実時間を待たない', () => {
    const clock = createMockClock(START);
    const firedAt: number[] = [];
    clock.setTimeout(() => firedAt.push(clock.now()), 60 * 60 * 1000);
    clock.advance(60 * 60 * 1000 - 1);
    expect(firedAt).toEqual([]);
    clock.advance(1);
    expect(firedAt).toEqual([START + 60 * 60 * 1000]);
    expect(clock.pendingTimers()).toBe(0);
  });

  it('1 回の advance の中で 期限の順に呼ばれ callback が見る時刻は その期限', () => {
    const clock = createMockClock(START);
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
    const clock = createMockClock(START);
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
    const clock = createMockClock(START);
    let fired = false;
    const handle = clock.setTimeout(() => {
      fired = true;
    }, 10);
    clock.clearTimeout(handle);
    clock.advance(100);
    expect(fired).toBe(false);
  });

  it('負の advance は 誤り', () => {
    const clock = createMockClock(START);
    expect(() => clock.advance(-1)).toThrow('non-negative');
    expect(() => clock.advance(Number.NaN)).toThrow('non-negative');
  });

  it('呼ばれた内容が読める', () => {
    const clock = createMockClock(START);
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

describe('REQ-OUT-MOCK mock の Http', () => {
  it('決めた応答を返し 送られた内容を 送られた順に記録する', async () => {
    const http = createMockHttp([
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
    const http = createMockHttp();
    await expect(
      http.request({ method: 'GET', url: 'https://api.example.test/unknown' }),
    ).rejects.toThrow('no response configured for GET https://api.example.test/unknown');
    expect(http.calls).toHaveLength(1);
  });
});

describe('REQ-OUT-MOCK mock の Storage', () => {
  it('空から始まり 別に作った物と 中身を分け合わない', () => {
    const first = createMockStorage();
    first.set('token', 'abc');
    const second = createMockStorage();
    expect(second.get('token')).toBeNull();
    expect(first.get('token')).toBe('abc');
  });

  it('呼ばれた内容が読める', () => {
    const storage = createMockStorage();
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

describe('REQ-OUT-MOCK mock の Config', () => {
  it('渡した表の値だけを返し 機体の環境変数は 結果に現れない', () => {
    const config = createMockConfig({ API_URL: 'https://api.example.test' });
    expect(config.get('API_URL')).toBe('https://api.example.test');
    // 多くの機体に在る名前でも 表に無ければ 無い。
    for (const name of ['PATH', 'HOME', 'USER', 'TZ', 'NODE_ENV']) {
      expect(config.get(name)).toBeUndefined();
    }
  });

  it('渡した表を後から書き換えても 結果は変わらない', () => {
    const table: Record<string, string> = { A: '1' };
    const config = createMockConfig(table);
    table['A'] = '2';
    expect(config.get('A')).toBe('1');
  });

  it('引かれた名前が読める', () => {
    const config = createMockConfig({ A: '1' });
    config.get('A');
    config.get('B');
    expect(config.calls).toEqual(['A', 'B']);
  });
});

describe('TC-OUT-IDGEN-MOCK mock の IdGen', () => {
  it('id-0001 から 呼んだ順に決まった ID を返し mock ごとに 1 から始まる', () => {
    const first = createMockIdGen();
    expect([first.newId(), first.newId(), first.newId()]).toEqual(['id-0001', 'id-0002', 'id-0003']);
    const second = createMockIdGen();
    expect(second.newId()).toBe('id-0001');
    expect(first.newId()).toBe('id-0004');
  });

  it('呼ばれた回数が読める', () => {
    const idGen = createMockIdGen();
    expect(idGen.calls).toBe(0);
    idGen.newId();
    idGen.newId();
    expect(idGen.calls).toBe(2);
  });
});

describe('TC-OUT-RANDOM-FE mock の Random', () => {
  const read = (random: ReturnType<typeof createMockRandom>, n: number): number[] =>
    Array.from(random.read(new Uint8Array(n)));

  it('同じ種・同じ呼び順なら 同じ byte', () => {
    const a = createMockRandom('seed-a');
    const b = createMockRandom('seed-a');
    expect([read(a, 16), read(a, 8)]).toEqual([read(b, 16), read(b, 8)]);
    const bytes = new Uint8Array([1, 2, 3]);
    expect(read(createMockRandom(bytes), 16)).toEqual(read(createMockRandom(new Uint8Array([1, 2, 3])), 16));
  });

  it('文字列の種は その UTF-8 の byte 列の種と同じ列', () => {
    expect(read(createMockRandom('種'), 16)).toEqual(
      read(createMockRandom(new TextEncoder().encode('種')), 16),
    );
  });

  it('違う種は 違う byte', () => {
    expect(read(createMockRandom('seed-a'), 24)).not.toEqual(read(createMockRandom('seed-b'), 24));
    expect(read(createMockRandom(''), 24)).not.toEqual(read(createMockRandom(new Uint8Array([0])), 24));
  });

  it('分けて読んでも 続けて読んでも 同じ列', () => {
    const whole = read(createMockRandom('stream'), 24);
    const split = createMockRandom('stream');
    expect([...read(split, 16), ...read(split, 8)]).toEqual(whole);
    const odd = createMockRandom('stream');
    expect([...read(odd, 3), ...read(odd, 0), ...read(odd, 5), ...read(odd, 16)]).toEqual(whole);
  });

  it('列は FNV-1a 64bit の種から始まる splitmix64 (dart の MockRandom と同じ列)', () => {
    expect(read(createMockRandom('outside'), 12)).toEqual([23, 1, 114, 187, 19, 129, 210, 158, 59, 154, 56, 168]);
    expect(read(createMockRandom(''), 4)).toEqual([48, 255, 164, 107]);
  });

  it('読んだ長さを 読んだ順に記録する', () => {
    const random = createMockRandom('calls');
    random.read(new Uint8Array(16));
    random.read(new Uint8Array(8));
    random.read(new Uint8Array(0));
    expect(random.calls).toEqual([16, 8, 0]);
  });

  it('withErr の後は その誤りを投げ 記録は残す。withErr(undefined) で 列の続きに戻る', () => {
    const whole = read(createMockRandom('err'), 16);
    const random = createMockRandom('err');
    const first = read(random, 8);
    const err = new Error('entropy exhausted');
    expect(random.withErr(err)).toBe(random);
    expect(() => random.read(new Uint8Array(4))).toThrow(err);
    expect(random.withErr(undefined)).toBe(random);
    expect([...first, ...read(random, 8)]).toEqual(whole);
    expect(random.calls).toEqual([8, 4, 8]);
  });
});

describe('REQ-OUT-MOCK mock の set', () => {
  it('渡した値から 6 つの mock を作る', async () => {
    const set = createMockSet({
      startMs: START,
      routes: [{ method: 'GET', url: 'https://api.example.test/', response: { status: 204 } }],
      config: { MODE: 'test' },
      seed: 'set-seed',
    });
    expect(set.clock.now()).toBe(START);
    expect((await set.http.request({ method: 'GET', url: 'https://api.example.test/' })).status).toBe(204);
    expect(set.storage.get('k')).toBeNull();
    expect(set.config.get('MODE')).toBe('test');
    expect(set.idGen.newId()).toBe('id-0001');
    expect(Array.from(set.random.read(new Uint8Array(8)))).toEqual(
      Array.from(createMockRandom('set-seed').read(new Uint8Array(8))),
    );
  });

  it('種を渡さなければ DEFAULT_MOCK_RANDOM_SEED の列', () => {
    const set = createMockSet({ startMs: START });
    expect(Array.from(set.random.read(new Uint8Array(8)))).toEqual(
      Array.from(createMockRandom(DEFAULT_MOCK_RANDOM_SEED).read(new Uint8Array(8))),
    );
  });
});
