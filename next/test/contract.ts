// 口ごとの契約。real と mock の両方に 同じ関数を掛ける。

import { describe, expect, it } from 'vitest';
import type { Clock, Config, Http, Storage } from '../src/index';

export interface ClockUnderTest {
  clock: Clock;
  /** 長さ 0 の timer が呼ばれる所まで 時を進める。 */
  pass(): Promise<void>;
}

export function clockContract(label: string, make: () => ClockUnderTest): void {
  describe(`TC-OUT-CONTRACT Clock (${label})`, () => {
    it('now は 整数の ms を返し 戻らない', async () => {
      const { clock, pass } = make();
      const a = clock.now();
      await pass();
      const b = clock.now();
      expect(Number.isInteger(a)).toBe(true);
      expect(b).toBeGreaterThanOrEqual(a);
    });

    it('setTimeout の callback は 同期では呼ばれず 時が進むと 1 回呼ばれる', async () => {
      const { clock, pass } = make();
      let count = 0;
      clock.setTimeout(() => {
        count += 1;
      }, 0);
      expect(count).toBe(0);
      await pass();
      expect(count).toBe(1);
      await pass();
      expect(count).toBe(1);
    });

    it('clearTimeout した timer は 呼ばれない', async () => {
      const { clock, pass } = make();
      const fired: string[] = [];
      const cancelled = clock.setTimeout(() => fired.push('cancelled'), 0);
      clock.setTimeout(() => fired.push('kept'), 0);
      clock.clearTimeout(cancelled);
      await pass();
      expect(fired).toEqual(['kept']);
    });

    it('同じ長さの timer は 掛けた順に呼ばれる', async () => {
      const { clock, pass } = make();
      const fired: number[] = [];
      clock.setTimeout(() => fired.push(1), 0);
      clock.setTimeout(() => fired.push(2), 0);
      clock.setTimeout(() => fired.push(3), 0);
      await pass();
      expect(fired).toEqual([1, 2, 3]);
    });

    it('済んだ timer の clearTimeout は 何もしない', async () => {
      const { clock, pass } = make();
      const handle = clock.setTimeout(() => {}, 0);
      await pass();
      expect(() => clock.clearTimeout(handle)).not.toThrow();
    });
  });
}

/** 契約が Http に決めさせる応答。 */
export type ContractRoute =
  | { method: string; url: string; status: number; headers: Record<string, string>; body: string }
  | { method: string; url: string; error: string };

export function httpContract(label: string, make: (routes: ContractRoute[]) => Http): void {
  describe(`TC-OUT-CONTRACT Http (${label})`, () => {
    it('status・header (名前は小文字)・body を返す', async () => {
      const http = make([
        {
          method: 'GET',
          url: 'https://api.example.test/items',
          status: 200,
          headers: { 'Content-Type': 'application/json', 'X-Trace': 'abc' },
          body: '{"items":[]}',
        },
      ]);
      const res = await http.request({ method: 'GET', url: 'https://api.example.test/items' });
      expect(res).toEqual({
        status: 200,
        headers: { 'content-type': 'application/json', 'x-trace': 'abc' },
        body: '{"items":[]}',
      });
    });

    it('2xx でない status も reject せず 応答として返す', async () => {
      const http = make([
        {
          method: 'POST',
          url: 'https://api.example.test/items',
          status: 404,
          headers: {},
          body: 'not found',
        },
      ]);
      const res = await http.request({
        method: 'POST',
        url: 'https://api.example.test/items',
        headers: { 'content-type': 'text/plain' },
        body: 'x',
      });
      expect(res.status).toBe(404);
      expect(res.body).toBe('not found');
    });

    it('body の無い応答は 空の文字列を返す', async () => {
      const http = make([
        { method: 'DELETE', url: 'https://api.example.test/items/1', status: 500, headers: {}, body: '' },
      ]);
      const res = await http.request({ method: 'DELETE', url: 'https://api.example.test/items/1' });
      expect(res).toEqual({ status: 500, headers: {}, body: '' });
    });

    it('応答が届かない時は reject する', async () => {
      const http = make([
        { method: 'GET', url: 'https://api.example.test/down', error: 'connection refused' },
      ]);
      await expect(
        http.request({ method: 'GET', url: 'https://api.example.test/down' }),
      ).rejects.toThrow('connection refused');
    });
  });
}

export function storageContract(label: string, make: () => Storage): void {
  describe(`TC-OUT-CONTRACT Storage (${label})`, () => {
    it('無い key は null', () => {
      expect(make().get('missing')).toBeNull();
    });

    it('書いた値が読める。書き直すと 新しい値が読める', () => {
      const storage = make();
      storage.set('k', 'v1');
      expect(storage.get('k')).toBe('v1');
      storage.set('k', 'v2');
      expect(storage.get('k')).toBe('v2');
    });

    it('空の文字列は 値として読める (null にならない)', () => {
      const storage = make();
      storage.set('empty', '');
      expect(storage.get('empty')).toBe('');
    });

    it('remove した key は null。他の key は 残る', () => {
      const storage = make();
      storage.set('a', '1');
      storage.set('b', '2');
      storage.remove('a');
      expect(storage.get('a')).toBeNull();
      expect(storage.get('b')).toBe('2');
    });

    it('無い key の remove は 何もしない', () => {
      const storage = make();
      expect(() => storage.remove('missing')).not.toThrow();
    });

    it('作り直すと 空から始まる', () => {
      make().set('k', 'v');
      expect(make().get('k')).toBeNull();
    });
  });
}

export function configContract(
  label: string,
  make: (table: Record<string, string>) => Config,
): void {
  describe(`TC-OUT-CONTRACT Config (${label})`, () => {
    it('表に在る名前は 値を返す', () => {
      const config = make({ API_URL: 'https://api.example.test', MODE: 'test' });
      expect(config.get('API_URL')).toBe('https://api.example.test');
      expect(config.get('MODE')).toBe('test');
    });

    it('表に無い名前は undefined', () => {
      expect(make({ API_URL: 'x' }).get('OTHER')).toBeUndefined();
    });

    it('空の文字列は 値として返す', () => {
      expect(make({ EMPTY: '' }).get('EMPTY')).toBe('');
    });

    it('object が元から持つ名前を 値として返さない', () => {
      const config = make({});
      expect(config.get('toString')).toBeUndefined();
      expect(config.get('constructor')).toBeUndefined();
      expect(config.get('__proto__')).toBeUndefined();
    });
  });
}
