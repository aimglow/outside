// real だけが持つ振る舞い (外へ渡す内容・外が無い時の誤り)。
// 外の物は test が渡すか test が global を差し替える。機体の物は読まない。

import { existsSync, mkdirSync, realpathSync, symlinkSync, writeFileSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  createRealClock,
  createRealConfig,
  createRealFS,
  createRealHttp,
  createRealProc,
  createRealIdGen,
  createRealRandom,
  createRealSet,
  createRealStorage,
  type FetchLike,
} from '../src/index';
import { writeScripts } from './programs';
import { testDir } from './workdir';

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
  it('values を env より先に引く', () => {
    const config = createRealConfig({ values: { A: 'from-values' }, env: { A: 'from-env', B: 'b' } });
    expect(config.get('A')).toBe('from-values');
    expect(config.get('B')).toBe('b');
  });

  it('env を渡さなければ 使う時に process.env を引く', () => {
    const config = createRealConfig();
    vi.stubGlobal('process', { env: { OUTSIDE_TEST_NAME: 'stubbed' } });
    expect(config.get('OUTSIDE_TEST_NAME')).toBe('stubbed');
    expect(config.get('OUTSIDE_TEST_OTHER')).toBeUndefined();
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

describe('REQ-OUT-REAL-TESTED-HERE real の Random', () => {
  it('使う時に global の crypto を取り getRandomValues を crypto の method として呼ぶ', () => {
    const random = createRealRandom();
    const stub = {
      fill: 7,
      getRandomValues(this: { fill: number }, array: Uint8Array) {
        array.fill(this.fill);
        return array;
      },
    };
    vi.stubGlobal('crypto', stub);
    expect(Array.from(random.read(new Uint8Array(3)))).toEqual([7, 7, 7]);
  });

  it('65536 byte より長い buf は 65536 byte までに分けて 全部埋める', () => {
    const random = createRealRandom();
    const lengths: number[] = [];
    let next = 1;
    vi.stubGlobal('crypto', {
      getRandomValues(array: Uint8Array) {
        if (array.length > 65536) throw new Error('QuotaExceededError');
        lengths.push(array.length);
        array.fill(next);
        next += 1;
        return array;
      },
    });
    const buf = random.read(new Uint8Array(65536 * 2 + 10));
    expect(lengths).toEqual([65536, 65536, 10]);
    expect(buf[0]).toBe(1);
    expect(buf[65536]).toBe(2);
    expect(buf[65536 * 2 + 9]).toBe(3);
  });

  it('実の getRandomValues で 65536 byte より長い buf を埋める', () => {
    const buf = createRealRandom().read(new Uint8Array(65536 + 64));
    expect(buf.subarray(65536).some((b) => b !== 0)).toBe(true);
  });

  it('crypto か getRandomValues が無い環境では 無い物を名乗って throw する', () => {
    const random = createRealRandom();
    vi.stubGlobal('crypto', undefined);
    expect(() => random.read(new Uint8Array(1))).toThrow('crypto is not available');
    vi.stubGlobal('crypto', {});
    expect(() => random.read(new Uint8Array(1))).toThrow('crypto.getRandomValues is not available');
  });
});

describe('REQ-OUT-INJECT real の set', () => {
  it('作るだけでは 外を触らない', () => {
    vi.stubGlobal('fetch', undefined);
    vi.stubGlobal('localStorage', undefined);
    vi.stubGlobal('process', undefined);
    vi.stubGlobal('crypto', undefined);
    const set = createRealSet();
    expect(Object.keys(set).sort()).toEqual(['clock', 'config', 'fs', 'http', 'idGen', 'proc', 'random', 'storage']);
  });
});

/** reject の誤りの code を返す (reject しなければ 'resolved')。 */
async function codeOf(p: Promise<unknown>): Promise<string> {
  try {
    await p;
    return 'resolved';
  } catch (err) {
    return String((err as { code?: unknown }).code);
  }
}

/** reject の誤りを返す (reject しなければ throw する)。 */
async function errorOf(p: Promise<unknown>): Promise<Error & { code?: string }> {
  try {
    await p;
  } catch (err) {
    return err as Error & { code?: string };
  }
  throw new Error('resolved');
}

describe('TC-OUT-NEXT-FS real の FS (symlink と 根)', () => {
  // 根と 根の外 (別の test の dir) を作り 根の中に symlink を置く。
  function setup() {
    const root = realpathSync(testDir());
    const away = realpathSync(testDir());
    writeFileSync(join(away, 'secret.txt'), 'secret');
    mkdirSync(join(root, 'in/sub'), { recursive: true });
    writeFileSync(join(root, 'in/sub/f.txt'), 'inside');
    symlinkSync('sub/f.txt', join(root, 'in/rel-file'));
    symlinkSync(join(root, 'in/sub'), join(root, 'abs-dir'));
    symlinkSync('../../in', join(root, 'in/sub/up'));
    symlinkSync(join(away, 'secret.txt'), join(root, 'out-abs'));
    symlinkSync(relative(root, join(away, 'secret.txt')), join(root, 'out-rel'));
    symlinkSync(away, join(root, 'dir-out'));
    symlinkSync(join(away, 'new.txt'), join(root, 'dangling-out'));
    symlinkSync('loop', join(root, 'loop'));
    return { root, away, fs: createRealFS(root) };
  }

  it('根の中に解ける symlink は辿る (相対・絶対・.. を含む target)', async () => {
    const { fs } = setup();
    expect(new TextDecoder().decode(await fs.readFile('in/rel-file'))).toBe('inside');
    expect(await fs.readDir('abs-dir')).toEqual([
      { name: 'f.txt', isDir: false },
      { name: 'up', isDir: false },
    ]);
    expect(await fs.stat('in/sub/up/sub/f.txt')).toEqual({ name: 'f.txt', size: 6, isDir: false });
    await fs.writeFile('abs-dir/new.txt', 'n');
    expect(new TextDecoder().decode(await fs.readFile('in/sub/new.txt'))).toBe('n');
  });

  it('根の外へ出る symlink は どの method も EOUTSIDEROOT で 根の外を触らない', async () => {
    const { fs, away } = setup();
    expect(await codeOf(fs.readFile('out-abs'))).toBe('EOUTSIDEROOT');
    expect(await codeOf(fs.readFile('out-rel'))).toBe('EOUTSIDEROOT');
    expect(await codeOf(fs.stat('out-rel'))).toBe('EOUTSIDEROOT');
    expect(await codeOf(fs.readDir('dir-out'))).toBe('EOUTSIDEROOT');
    expect(await codeOf(fs.readFile('dir-out/secret.txt'))).toBe('EOUTSIDEROOT');
    expect(await codeOf(fs.writeFile('dir-out/new.txt', 'x'))).toBe('EOUTSIDEROOT');
    expect(await codeOf(fs.writeFile('dangling-out', 'x'))).toBe('EOUTSIDEROOT');
    expect(await codeOf(fs.writeFile('out-abs', 'x'))).toBe('EOUTSIDEROOT');
    expect(await codeOf(fs.mkdirAll('dir-out/sub'))).toBe('EOUTSIDEROOT');
    expect(await codeOf(fs.remove('dir-out/secret.txt'))).toBe('EOUTSIDEROOT');
    expect(existsSync(join(away, 'new.txt'))).toBe(false);
    expect(existsSync(join(away, 'sub'))).toBe(false);
    expect(new TextDecoder().decode(await import('node:fs/promises').then((m) => m.readFile(join(away, 'secret.txt'))))).toBe('secret');
  });

  it('remove は 最後の名前の symlink を辿らず link だけを消す', async () => {
    const { fs, root, away } = setup();
    await fs.remove('out-abs');
    await fs.remove('dir-out');
    expect(existsSync(join(root, 'out-abs'))).toBe(false);
    expect(existsSync(join(root, 'dir-out'))).toBe(false);
    expect(existsSync(join(away, 'secret.txt'))).toBe(true);
  });

  it('自分を指す symlink は ELOOP', async () => {
    const { fs } = setup();
    expect(await codeOf(fs.readFile('loop'))).toBe('ELOOP');
  });

  it('根は絶対 path でなければ 作る時に throw する。無い根は 使う時に ENOENT', async () => {
    expect(() => createRealFS('relative/root')).toThrow('absolute');
    const fs = createRealFS(join(testDir(), 'missing-root'));
    expect(await codeOf(fs.readDir('.'))).toBe('ENOENT');
  });

  it('誤りの文は 根の外の絶対 path を名乗らない (相対 path で名乗る)', async () => {
    const root = realpathSync(testDir());
    const err = await errorOf(createRealFS(root).readFile('a/missing.txt'));
    expect(err.message).toContain('a/missing.txt');
    expect(err.message).not.toContain(root);
  });
});

describe('REQ-OUT-INJECT real の set の FS と Proc', () => {
  it('fsRoot を渡さなければ fs は 使った時に 根が宣言されていないと名乗って ENOROOT で reject する', async () => {
    const set = createRealSet();
    for (const run of [
      () => set.fs.readFile('a'),
      () => set.fs.writeFile('a', 'x'),
      () => set.fs.readDir('.'),
      () => set.fs.stat('a'),
      () => set.fs.remove('a'),
      () => set.fs.mkdirAll('a'),
    ]) {
      await expect(run()).rejects.toMatchObject({ code: 'ENOROOT', message: expect.stringContaining('not declared') });
    }
  });

  it('fsRoot を渡すと その根の FS', async () => {
    const root = testDir();
    writeFileSync(join(root, 'a.txt'), 'a');
    const set = createRealSet({ fsRoot: root });
    expect(new TextDecoder().decode(await set.fs.readFile('a.txt'))).toBe('a');
  });

  it('programs を渡さなければ proc は 全ての名前が EUNDECLARED。渡すと その名前を起こす', async () => {
    await expect(createRealSet().proc.run({ name: 'node', args: [] })).rejects.toMatchObject({ code: 'EUNDECLARED' });
    const scripts = writeScripts(testDir());
    const res = await createRealSet({ programs: { node: process.execPath } }).proc.run({
      name: 'node',
      args: [scripts.echo, 'set'],
    });
    expect(new TextDecoder().decode(res.stdout)).toBe('set\n');
  });
});

describe('TC-OUT-NEXT-PROC real の Proc (実の子 process)', () => {
  it('宣言の path に program が無ければ 名前を名乗って ENOENT で reject する', async () => {
    const missing = join(testDir(), 'no-such-program');
    const err = await errorOf(createRealProc({ tool: missing }).run({ name: 'tool', args: [] }));
    expect(err.code).toBe('ENOENT');
    expect(err.message).toContain('tool');
  });

  it('空の path は 子の環境変数の PATH から探す', async () => {
    const scripts = writeScripts(testDir());
    const res = await createRealProc({ node: '' }).run({
      name: 'node',
      args: [scripts.echo, 'from-path'],
      env: { PATH: dirname(process.execPath) },
    });
    expect(new TextDecoder().decode(res.stdout)).toBe('from-path\n');
  });

  it('走っている子を abort で止める (子は居なくなる)', async () => {
    const dir = testDir();
    const scripts = writeScripts(dir);
    const marker = join(dir, 'started');
    const controller = new AbortController();
    const run = createRealProc({ node: process.execPath }).run({
      name: 'node',
      args: [scripts.hang, marker],
      signal: controller.signal,
    });
    // 子が起きた印 (pid を書いた file) を待つ。上限は 回数で切る (時計を読まない)。
    for (let i = 0; i < 1000 && !existsSync(marker); i += 1) {
      await new Promise((done) => setTimeout(done, 5));
    }
    expect(existsSync(marker)).toBe(true);
    const pid = Number(await import('node:fs/promises').then((m) => m.readFile(marker, 'utf8')));
    controller.abort();
    await expect(run).rejects.toMatchObject({ name: 'AbortError', code: 'ABORT_ERR' });
    const alive = (): boolean => {
      try {
        process.kill(pid, 0);
        return true;
      } catch {
        return false;
      }
    };
    for (let i = 0; i < 1000 && alive(); i += 1) {
      await new Promise((done) => setTimeout(done, 5));
    }
    expect(alive()).toBe(false);
  });

  it('子が signal で終わった時は 名乗って reject する (exitCode が無い)', async () => {
    const dir = testDir();
    const script = join(dir, 'self-kill.cjs');
    writeFileSync(script, "process.kill(process.pid, 'SIGKILL');\n");
    await expect(
      createRealProc({ node: process.execPath }).run({ name: 'node', args: [script] }),
    ).rejects.toMatchObject({ code: 'ESIGNAL', message: expect.stringContaining('SIGKILL') });
  });
});
