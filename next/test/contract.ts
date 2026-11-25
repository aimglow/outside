// 口ごとの契約。real と mock の両方に 同じ関数を掛ける。

import { describe, expect, it } from 'vitest';
import type { Clock, Config, FS, Http, IdGen, Proc, Random, Storage } from '../src/index';

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

export function idGenContract(label: string, make: () => IdGen): void {
  describe(`TC-OUT-CONTRACT IdGen (${label})`, () => {
    it('続けて呼んだ ID は 互いに違い 空でない', () => {
      const idGen = make();
      const ids = Array.from({ length: 100 }, () => idGen.newId());
      for (const id of ids) {
        expect(typeof id).toBe('string');
        expect(id.length).toBeGreaterThan(0);
      }
      expect(new Set(ids).size).toBe(ids.length);
    });
  });
}

export function randomContract(label: string, make: () => Random): void {
  describe(`TC-OUT-RANDOM-FE Random (${label})`, () => {
    it('渡した buf を埋めて 同じ buf を返す。続けて読んだ 16 byte は 互いに違う', () => {
      const random = make();
      const first = new Uint8Array(16);
      const second = new Uint8Array(8);
      const third = new Uint8Array(16);
      expect(random.read(first)).toBe(first);
      expect(random.read(second)).toBe(second);
      expect(random.read(third)).toBe(third);
      expect(first.some((b) => b !== 0)).toBe(true);
      expect(second.some((b) => b !== 0)).toBe(true);
      expect(Array.from(first)).not.toEqual(Array.from(third));
    });

    it('長さ 0 の buf は そのまま返す', () => {
      const empty = new Uint8Array(0);
      expect(make().read(empty)).toBe(empty);
    });

    it('buf の一部 (subarray) を渡すと その範囲だけを埋める', () => {
      const whole = new Uint8Array(48);
      make().read(whole.subarray(16, 32));
      expect(Array.from(whole.subarray(0, 16)).every((b) => b === 0)).toBe(true);
      expect(Array.from(whole.subarray(32)).every((b) => b === 0)).toBe(true);
      expect(whole.subarray(16, 32).some((b) => b !== 0)).toBe(true);
    });
  });
}

const bytes = (s: string): Uint8Array => new TextEncoder().encode(s);
const text = (b: Uint8Array): string => new TextDecoder().decode(b);

/** reject の誤りの code を返す (reject しなければ 'resolved')。 */
async function codeOf(p: Promise<unknown>): Promise<string> {
  try {
    await p;
    return 'resolved';
  } catch (err) {
    return String((err as { code?: unknown }).code);
  }
}

/** 契約が FS に渡す木 (根からの相対 path → 中身、末尾 / は 空の dir)。 */
export type ContractTree = Record<string, string | Uint8Array>;

export function fsContract(label: string, make: (tree: ContractTree) => FS | Promise<FS>): void {
  describe(`TC-OUT-NEXT-FS FS (${label})`, () => {
    it('書いた file を読める (Uint8Array と string)。書き直すと 新しい中身', async () => {
      const fs = await make({});
      const raw = new Uint8Array([0, 1, 2, 254, 255]);
      await fs.writeFile('bin.dat', raw);
      expect(await fs.readFile('bin.dat')).toEqual(raw);
      await fs.writeFile('note.txt', 'こんにちは\n');
      expect(text(await fs.readFile('note.txt'))).toBe('こんにちは\n');
      await fs.writeFile('note.txt', 'x');
      expect(text(await fs.readFile('note.txt'))).toBe('x');
    });

    it('readFile は 写しを返す (返した byte を変えても 中身は変わらない)', async () => {
      const fs = await make({ 'a.txt': 'abc' });
      const got = await fs.readFile('a.txt');
      expect(got).toBeInstanceOf(Uint8Array);
      got[0] = 0;
      expect(text(await fs.readFile('a.txt'))).toBe('abc');
    });

    it('親の無い path への writeFile は 親の dir を作る', async () => {
      const fs = await make({});
      await fs.writeFile('a/b/c.txt', 'c');
      expect(await fs.readDir('a')).toEqual([{ name: 'b', isDir: true }]);
      expect(text(await fs.readFile('a/b/c.txt'))).toBe('c');
    });

    it('readDir は 名前の順 (code unit の順) の {name, isDir}', async () => {
      const fs = await make({ 'b.txt': 'b', 'a.txt': 'a', 'C.txt': 'c', 'dir/': '', 'dir2/x': 'x' });
      expect(await fs.readDir('.')).toEqual([
        { name: 'C.txt', isDir: false },
        { name: 'a.txt', isDir: false },
        { name: 'b.txt', isDir: false },
        { name: 'dir', isDir: true },
        { name: 'dir2', isDir: true },
      ]);
      expect(await fs.readDir('dir')).toEqual([]);
    });

    it('stat は file と dir の {name, size, isDir} (dir の size は 0、根の name は .)', async () => {
      const fs = await make({ 'd/f.bin': new Uint8Array(7), 'e/': '' });
      expect(await fs.stat('d/f.bin')).toEqual({ name: 'f.bin', size: 7, isDir: false });
      expect(await fs.stat('d')).toEqual({ name: 'd', size: 0, isDir: true });
      expect(await fs.stat('e')).toEqual({ name: 'e', size: 0, isDir: true });
      expect(await fs.stat('.')).toEqual({ name: '.', size: 0, isDir: true });
    });

    it('path の . と 重なった / と 中の .. は 整えてから使う', async () => {
      const fs = await make({ 'a/b/c.txt': 'c' });
      expect(text(await fs.readFile('a/./b//c.txt'))).toBe('c');
      expect(text(await fs.readFile('a/x/../b/c.txt'))).toBe('c');
      expect((await fs.stat('a/b/')).isDir).toBe(true);
    });

    it('無い path は code ENOENT', async () => {
      const fs = await make({ 'a.txt': 'a' });
      expect(await codeOf(fs.readFile('missing.txt'))).toBe('ENOENT');
      expect(await codeOf(fs.stat('missing'))).toBe('ENOENT');
      expect(await codeOf(fs.readDir('missing'))).toBe('ENOENT');
      expect(await codeOf(fs.remove('missing'))).toBe('ENOENT');
      expect(await codeOf(fs.readFile('missing/x.txt'))).toBe('ENOENT');
    });

    it('根の外 (.. で出る・絶対 path・空) は どの method も EOUTSIDEROOT で 何も触らない', async () => {
      const fs = await make({ 'a/b.txt': 'b' });
      for (const p of ['../x', '..', 'a/../../x', '/etc/hosts', '\\x', 'C:/x', '']) {
        expect(await codeOf(fs.readFile(p)), `readFile ${p}`).toBe('EOUTSIDEROOT');
        expect(await codeOf(fs.writeFile(p, 'x')), `writeFile ${p}`).toBe('EOUTSIDEROOT');
        expect(await codeOf(fs.readDir(p)), `readDir ${p}`).toBe('EOUTSIDEROOT');
        expect(await codeOf(fs.stat(p)), `stat ${p}`).toBe('EOUTSIDEROOT');
        expect(await codeOf(fs.remove(p)), `remove ${p}`).toBe('EOUTSIDEROOT');
        expect(await codeOf(fs.mkdirAll(p)), `mkdirAll ${p}`).toBe('EOUTSIDEROOT');
      }
      expect(await fs.readDir('.')).toEqual([{ name: 'a', isDir: true }]);
    });

    it('remove は file と 空の dir を消す。中身の在る dir は ENOTEMPTY、根は EPERM', async () => {
      const fs = await make({ 'f.txt': 'f', 'empty/': '', 'full/x.txt': 'x' });
      await fs.remove('f.txt');
      await fs.remove('empty');
      expect(await codeOf(fs.stat('f.txt'))).toBe('ENOENT');
      expect(await codeOf(fs.stat('empty'))).toBe('ENOENT');
      expect(await codeOf(fs.remove('full'))).toBe('ENOTEMPTY');
      expect(text(await fs.readFile('full/x.txt'))).toBe('x');
      expect(await codeOf(fs.remove('.'))).toBe('EPERM');
      expect(await codeOf(fs.remove('a/..'))).toBe('EPERM');
      expect(await fs.readDir('.')).toEqual([{ name: 'full', isDir: true }]);
    });

    it('mkdirAll は 入れ子の dir を作り 在れば何もしない。file の所は EEXIST、file の下は ENOTDIR', async () => {
      const fs = await make({ 'f.txt': 'f', 'has/x.txt': 'x' });
      await fs.mkdirAll('a/b/c');
      expect(await fs.stat('a/b/c')).toEqual({ name: 'c', size: 0, isDir: true });
      await fs.mkdirAll('a/b/c');
      await fs.mkdirAll('has');
      await fs.mkdirAll('.');
      expect(await fs.readDir('has')).toEqual([{ name: 'x.txt', isDir: false }]);
      expect(await codeOf(fs.mkdirAll('f.txt'))).toBe('EEXIST');
      expect(await codeOf(fs.mkdirAll('f.txt/sub'))).toBe('ENOTDIR');
    });

    it('種類の合わない操作: dir の readFile・writeFile は EISDIR、file の readDir と file の下は ENOTDIR', async () => {
      const fs = await make({ 'd/x.txt': 'x', 'f.txt': 'f' });
      expect(await codeOf(fs.readFile('d'))).toBe('EISDIR');
      expect(await codeOf(fs.writeFile('d', 'x'))).toBe('EISDIR');
      expect(await codeOf(fs.writeFile('.', 'x'))).toBe('EISDIR');
      expect(await codeOf(fs.readDir('f.txt'))).toBe('ENOTDIR');
      expect(await codeOf(fs.readFile('f.txt/y'))).toBe('ENOTDIR');
      expect(await codeOf(fs.writeFile('f.txt/y', 'y'))).toBe('ENOTDIR');
      expect(await codeOf(fs.stat('f.txt/y'))).toBe('ENOTDIR');
      expect(text(await fs.readFile('f.txt'))).toBe('f');
    });

    it('木の宣言の Uint8Array と string は 同じく byte で読める', async () => {
      const fs = await make({ 'u.bin': new Uint8Array([9, 8]), 's.txt': 'é' });
      expect(await fs.readFile('u.bin')).toEqual(new Uint8Array([9, 8]));
      expect(await fs.readFile('s.txt')).toEqual(bytes('é'));
    });
  });
}

/** 契約が使う program の振る舞いの名前。real は 同じ名前の node の script、mock は 同じ振る舞いの関数。 */
export type ContractBehavior = 'echo' | 'fail' | 'cat' | 'env' | 'pwd' | 'hang';

export interface ProcUnderTest {
  proc: Proc;
  /** 宣言した program の名前 (振る舞いは 最初の引数で選ぶ)。 */
  name: string;
  /** 振る舞いを選ぶ 最初の引数。 */
  select(behavior: ContractBehavior): string[];
  /** 子の作業 dir に渡せる dir。 */
  dir: string;
  /** 親の環境変数に在る 名前と値 (test が置いた物)。 */
  parentEnv: { name: string; value: string };
}

export function procContract(label: string, make: () => ProcUnderTest | Promise<ProcUnderTest>): void {
  describe(`TC-OUT-NEXT-PROC Proc (${label})`, () => {
    it('stdout と exitCode 0 を返す (stdout・stderr は Uint8Array)', async () => {
      const h = await make();
      const res = await h.proc.run({ name: h.name, args: [...h.select('echo'), 'hello', 'こんにちは'] });
      expect(res.exitCode).toBe(0);
      expect(res.stdout).toBeInstanceOf(Uint8Array);
      expect(res.stderr).toBeInstanceOf(Uint8Array);
      expect(text(res.stdout)).toBe('hello こんにちは\n');
      expect(res.stderr.length).toBe(0);
    });

    it('引数は shell を通さず そのまま渡る', async () => {
      const h = await make();
      const res = await h.proc.run({ name: h.name, args: [...h.select('echo'), '$HOME', '`x`', 'a b;c'] });
      expect(text(res.stdout)).toBe('$HOME `x` a b;c\n');
    });

    it('0 以外の exit は reject せず 結果で返る (stderr も)', async () => {
      const h = await make();
      const res = await h.proc.run({ name: h.name, args: h.select('fail') });
      expect(res.exitCode).toBe(3);
      expect(text(res.stderr)).toBe('bad\n');
      expect(res.stdout.length).toBe(0);
    });

    it('stdin (string と Uint8Array) を 子の標準入力に渡す。無ければ 空の入力', async () => {
      const h = await make();
      const s = await h.proc.run({ name: h.name, args: h.select('cat'), stdin: 'secret-値' });
      expect(text(s.stdout)).toBe('secret-値');
      const raw = new Uint8Array([0, 10, 255, 128]);
      const b = await h.proc.run({ name: h.name, args: h.select('cat'), stdin: raw });
      expect(b.stdout).toEqual(raw);
      const none = await h.proc.run({ name: h.name, args: h.select('cat') });
      expect(none.stdout.length).toBe(0);
    });

    it('子の環境変数 = 親から unset を外し env を足した物 (env が unset より後)', async () => {
      const h = await make();
      const p = h.parentEnv.name;
      const read = async (cmd: { env?: Record<string, string>; unset?: string[] }) =>
        JSON.parse(
          text((await h.proc.run({ name: h.name, args: [...h.select('env'), p, 'OUTSIDE_TEST_ADDED'], ...cmd })).stdout),
        ) as Record<string, string | null>;
      expect(await read({})).toEqual({ [p]: h.parentEnv.value, OUTSIDE_TEST_ADDED: null });
      expect(await read({ env: { OUTSIDE_TEST_ADDED: 'a' } })).toEqual({ [p]: h.parentEnv.value, OUTSIDE_TEST_ADDED: 'a' });
      expect(await read({ unset: [p] })).toEqual({ [p]: null, OUTSIDE_TEST_ADDED: null });
      expect(await read({ unset: [p], env: { [p]: 'again' } })).toEqual({ [p]: 'again', OUTSIDE_TEST_ADDED: null });
    });

    it('dir を 子の作業 dir にする', async () => {
      const h = await make();
      const res = await h.proc.run({ name: h.name, args: h.select('pwd'), dir: h.dir });
      expect(text(res.stdout)).toBe(`${h.dir}\n`);
    });

    it('宣言に無い名前は 名前を名乗って EUNDECLARED で reject する', async () => {
      const h = await make();
      const run = h.proc.run({ name: 'not-declared', args: [] });
      await expect(run).rejects.toThrow('not-declared');
      expect(await codeOf(h.proc.run({ name: 'not-declared', args: [] }))).toBe('EUNDECLARED');
      expect(await codeOf(h.proc.run({ name: 'toString', args: [] }))).toBe('EUNDECLARED');
    });

    it('signal の abort で 子を止めて AbortError で reject する。abort 済みの signal は 起こさずに reject', async () => {
      const h = await make();
      const controller = new AbortController();
      const run = h.proc.run({ name: h.name, args: h.select('hang'), signal: controller.signal });
      controller.abort();
      await expect(run).rejects.toMatchObject({ name: 'AbortError', code: 'ABORT_ERR' });
      await expect(
        h.proc.run({ name: h.name, args: h.select('echo'), signal: AbortSignal.abort() }),
      ).rejects.toMatchObject({ name: 'AbortError', code: 'ABORT_ERR' });
    });

    it('済んだ後の abort は 結果を変えない', async () => {
      const h = await make();
      const controller = new AbortController();
      const res = await h.proc.run({ name: h.name, args: [...h.select('echo'), 'x'], signal: controller.signal });
      controller.abort();
      expect(text(res.stdout)).toBe('x\n');
    });
  });
}
