// outside のmock (mock)。口 1 つにつき 1 つ。
// test が渡した値だけで振る舞いが決まる。実の時計・環境変数・通信・保存・file system・process を読まない。
// 呼ばれた内容は calls から読める。

import type {
  Clock,
  Config,
  FS,
  FsDirEntry,
  Http,
  HttpRequest,
  IdGen,
  HttpResponse,
  OutsideSet,
  Proc,
  ProcCmd,
  ProcResult,
  Random,
  Storage,
  TimerHandle,
} from '../index';
import {
  OUTSIDE_ROOT,
  abortError,
  baseName,
  byName,
  cleanRel,
  fsError,
  outsideError,
  parentOf,
  rootRemoval,
  toBytes,
  undeclaredProgram,
} from '../rule';

export type MockClockCall =
  | { method: 'now' }
  | { method: 'setTimeout'; ms: number }
  | { method: 'clearTimeout' };

export interface MockClock extends Clock {
  /** 時刻を ms 進め その間に期限が来た timer を 期限の順 (同じ期限は 掛けた順) に呼ぶ。 */
  advance(ms: number): void;
  /** まだ呼ばれていない timer の数。 */
  pendingTimers(): number;
  readonly calls: readonly MockClockCall[];
}

interface MockTimer {
  id: number;
  due: number;
  fn: () => void;
}

/** startMs (epoch からの ms) から始まり advance でだけ進む Clock。 */
export function createMockClock(startMs: number): MockClock {
  let current = startMs;
  let nextId = 1;
  let timers: MockTimer[] = [];
  const calls: MockClockCall[] = [];

  // 期限が limit 以下の timer のうち 最も早い物を取り出す。
  const takeNext = (limit: number): MockTimer | undefined => {
    let best: MockTimer | undefined;
    for (const t of timers) {
      if (t.due > limit) continue;
      if (best === undefined || t.due < best.due || (t.due === best.due && t.id < best.id)) {
        best = t;
      }
    }
    if (best !== undefined) {
      const taken = best;
      timers = timers.filter((t) => t !== taken);
    }
    return best;
  };

  return {
    calls,
    now() {
      calls.push({ method: 'now' });
      return current;
    },
    setTimeout(fn, ms) {
      calls.push({ method: 'setTimeout', ms });
      const id = nextId++;
      // 負の長さと NaN は 0 として扱う (実の timer と同じ)。
      timers.push({ id, due: current + (ms > 0 ? ms : 0), fn });
      return id;
    },
    clearTimeout(handle: TimerHandle) {
      calls.push({ method: 'clearTimeout' });
      timers = timers.filter((t) => t.id !== handle);
    },
    advance(ms) {
      if (!(ms >= 0)) {
        throw new Error(`outside mock Clock: advance needs a non-negative number, got ${ms}`);
      }
      const end = current + ms;
      // callback の中で掛けられた timer も 期限が end 以下なら この advance の中で呼ぶ。
      for (let t = takeNext(end); t !== undefined; t = takeNext(end)) {
        current = t.due;
        t.fn();
      }
      current = end;
    },
    pendingTimers: () => timers.length,
  };
}

/** method と url が一致した request に返す物。response か error の どちらか。 */
export type MockHttpRoute =
  | { method: string; url: string; response: { status: number; headers?: Record<string, string>; body?: string } }
  | { method: string; url: string; error: string };

export interface MockHttp extends Http {
  /** 送られた request (送られた順)。 */
  readonly calls: readonly HttpRequest[];
}

/** routes に決めた応答を返す Http。routes に無い request と error の route は reject する。 */
export function createMockHttp(routes: readonly MockHttpRoute[] = []): MockHttp {
  const calls: HttpRequest[] = [];
  return {
    calls,
    request(req) {
      calls.push({ ...req, ...(req.headers ? { headers: { ...req.headers } } : {}) });
      const route = routes.find((r) => r.method === req.method && r.url === req.url);
      if (route === undefined) {
        return Promise.reject(
          new Error(`outside mock Http: no response configured for ${req.method} ${req.url}`),
        );
      }
      if ('error' in route) return Promise.reject(new Error(route.error));
      const headers: Record<string, string> = {};
      for (const [name, value] of Object.entries(route.response.headers ?? {})) {
        headers[name.toLowerCase()] = value;
      }
      const res: HttpResponse = {
        status: route.response.status,
        headers,
        body: route.response.body ?? '',
      };
      return Promise.resolve(res);
    },
  };
}

export type MockStorageCall =
  | { method: 'get'; key: string }
  | { method: 'set'; key: string; value: string }
  | { method: 'remove'; key: string };

export interface MockStorage extends Storage {
  readonly calls: readonly MockStorageCall[];
}

/** memory の上の表に保存する Storage。空から始まる。 */
export function createMockStorage(): MockStorage {
  const table = new Map<string, string>();
  const calls: MockStorageCall[] = [];
  return {
    calls,
    get(key) {
      calls.push({ method: 'get', key });
      return table.get(key) ?? null;
    },
    set(key, value) {
      calls.push({ method: 'set', key, value });
      table.set(key, value);
    },
    remove(key) {
      calls.push({ method: 'remove', key });
      table.delete(key);
    },
  };
}

export interface MockConfig extends Config {
  /** 引かれた名前 (引かれた順)。 */
  readonly calls: readonly string[];
}

/** 渡した表だけを返す Config。表に無い名前は undefined。 */
export function createMockConfig(table: Readonly<Record<string, string>> = {}): MockConfig {
  const values = new Map(Object.entries(table));
  const calls: string[] = [];
  return {
    calls,
    get(name) {
      calls.push(name);
      return values.get(name);
    },
  };
}

export interface MockIdGen extends IdGen {
  /** newId が呼ばれた回数。 */
  readonly calls: number;
}

/** 'id-0001'・'id-0002' … と 呼んだ順に決まった ID を返す IdGen。作るたびに 1 から始まる。 */
export function createMockIdGen(): MockIdGen {
  let count = 0;
  return {
    get calls() {
      return count;
    },
    newId() {
      count += 1;
      return `id-${String(count).padStart(4, '0')}`;
    },
  };
}

export interface MockRandom extends Random {
  /** read に渡された buf の長さ (読んだ順)。 */
  readonly calls: readonly number[];
  /** 以後の read が err を投げるようにする。undefined を渡すと 列を返す形に戻す。 */
  withErr(err: Error | undefined): MockRandom;
}

/** createMockSet が 種を渡されない時に使う種。 */
export const DEFAULT_MOCK_RANDOM_SEED = 'outside-mock-random';

const MASK64 = (1n << 64n) - 1n;

/**
 * 種で決まる byte 列を返す Random。同じ種・同じ呼び順なら 同じ byte。
 * 列 = 種の byte 列の FNV-1a 64bit を初期値とする splitmix64 の出力を 1 つずつ 8 byte (little endian) に開いて並べた物。
 * 1 本の byte の流れとして持つので 分けて読んでも 続けて読んでも 同じ列。文字列の種は UTF-8 の byte 列にする。
 */
export function createMockRandom(seed: Uint8Array | string): MockRandom {
  const seedBytes = typeof seed === 'string' ? new TextEncoder().encode(seed) : seed;
  let state = 0xcbf29ce484222325n;
  for (const b of seedBytes) {
    state = ((state ^ BigInt(b)) * 0x100000001b3n) & MASK64;
  }
  // 今の 8 byte の塊と その中で次に返す位置 (8 なら 次の塊を作る)。
  const block = new Uint8Array(8);
  let pos = 8;
  const nextBlock = (): void => {
    state = (state + 0x9e3779b97f4a7c15n) & MASK64;
    let z = state;
    z = ((z ^ (z >> 30n)) * 0xbf58476d1ce4e5b9n) & MASK64;
    z = ((z ^ (z >> 27n)) * 0x94d049bb133111ebn) & MASK64;
    z ^= z >> 31n;
    for (let i = 0; i < 8; i += 1) {
      block[i] = Number((z >> BigInt(8 * i)) & 0xffn);
    }
    pos = 0;
  };
  const calls: number[] = [];
  let failure: Error | undefined;
  const mock: MockRandom = {
    calls,
    withErr(err) {
      failure = err;
      return mock;
    },
    read(buf) {
      calls.push(buf.length);
      if (failure !== undefined) throw failure;
      for (let i = 0; i < buf.length; i += 1) {
        if (pos === 8) nextBlock();
        buf[i] = block[pos] ?? 0;
        pos += 1;
      }
      return buf;
    },
  };
  return mock;
}

export type MockFsCall = {
  method: 'readFile' | 'writeFile' | 'readDir' | 'stat' | 'remove' | 'mkdirAll';
  path: string;
};

export interface MockFS extends FS {
  /** 呼ばれた method と 渡された path (呼ばれた順)。 */
  readonly calls: readonly MockFsCall[];
}

/** mock の FS の木。根からの相対 path → 中身 (string は UTF-8)。空の dir は 末尾 / の path。 */
export type MockFsTree = Readonly<Record<string, string | Uint8Array>>;

/**
 * memory の上の木の FS。disk に木を作らない。
 * tree の path は 根からの相対。絶対 path・根の外・file と dir の食い違いの宣言は throw する。
 */
export function createMockFS(tree: MockFsTree = {}): MockFS {
  const files = new Map<string, Uint8Array>();
  const dirs = new Set<string>(['.']);
  const calls: MockFsCall[] = [];

  // rel までの dir を作る。途中に file が在れば ENOTDIR。
  const mkdirs = (op: string, rel: string, target: string): void => {
    if (rel === '.') return;
    mkdirs(op, parentOf(rel), target);
    if (files.has(rel)) throw fsError('ENOTDIR', op, target);
    dirs.add(rel);
  };

  for (const [key, value] of Object.entries(tree)) {
    const isDir = key.endsWith('/');
    let rel: string;
    try {
      rel = cleanRel(isDir ? key.slice(0, -1) : key);
    } catch {
      throw outsideError(OUTSIDE_ROOT, `outside mock FS: tree path must stay inside the root: ${key}`);
    }
    if (isDir) {
      if (files.has(rel)) throw new Error(`outside mock FS: tree declares ${rel} as both a file and a dir`);
      mkdirs('tree', rel, rel);
      continue;
    }
    if (rel === '.' || dirs.has(rel)) {
      throw new Error(`outside mock FS: tree declares ${rel} as both a file and a dir`);
    }
    try {
      mkdirs('tree', parentOf(rel), rel);
    } catch {
      throw new Error(`outside mock FS: tree declares a parent of ${rel} as a file`);
    }
    files.set(rel, toBytes(value));
  }

  // rel の親の dir を確かめる。途中が file なら ENOTDIR。create でなければ 無い dir は ENOENT。
  const checkParents = (op: string, rel: string, create: boolean): void => {
    const parts = rel === '.' ? [] : rel.split('/');
    for (let i = 1; i < parts.length; i += 1) {
      const prefix = parts.slice(0, i).join('/');
      if (files.has(prefix)) throw fsError('ENOTDIR', op, rel);
      if (!dirs.has(prefix)) {
        if (create) return;
        throw fsError('ENOENT', op, rel);
      }
    }
  };

  const children = (dir: string): FsDirEntry[] => {
    const out: FsDirEntry[] = [];
    const take = (name: string, isDir: boolean): void => {
      if (name !== '.' && parentOf(name) === dir) out.push({ name: baseName(name), isDir });
    };
    for (const name of files.keys()) take(name, false);
    for (const name of dirs) take(name, true);
    return out.sort(byName);
  };

  const start = (method: MockFsCall['method'], path: string): string => {
    calls.push({ method, path });
    return cleanRel(path);
  };

  return {
    calls,
    async readFile(path) {
      const rel = start('readFile', path);
      checkParents('readFile', rel, false);
      const data = files.get(rel);
      if (data !== undefined) return new Uint8Array(data);
      throw fsError(dirs.has(rel) ? 'EISDIR' : 'ENOENT', 'readFile', rel);
    },
    async writeFile(path, data) {
      const rel = start('writeFile', path);
      if (rel === '.' || dirs.has(rel)) throw fsError('EISDIR', 'writeFile', rel);
      checkParents('writeFile', rel, true);
      mkdirs('writeFile', parentOf(rel), rel);
      files.set(rel, toBytes(data));
    },
    async readDir(path) {
      const rel = start('readDir', path);
      checkParents('readDir', rel, false);
      if (files.has(rel)) throw fsError('ENOTDIR', 'readDir', rel);
      if (!dirs.has(rel)) throw fsError('ENOENT', 'readDir', rel);
      return children(rel);
    },
    async stat(path) {
      const rel = start('stat', path);
      checkParents('stat', rel, false);
      const data = files.get(rel);
      if (data !== undefined) return { name: baseName(rel), size: data.length, isDir: false };
      if (dirs.has(rel)) return { name: baseName(rel), size: 0, isDir: true };
      throw fsError('ENOENT', 'stat', rel);
    },
    async remove(path) {
      const rel = start('remove', path);
      if (rel === '.') throw rootRemoval();
      checkParents('remove', rel, false);
      if (files.delete(rel)) return;
      if (!dirs.has(rel)) throw fsError('ENOENT', 'remove', rel);
      if (children(rel).length > 0) throw fsError('ENOTEMPTY', 'remove', rel);
      dirs.delete(rel);
    },
    async mkdirAll(path) {
      const rel = start('mkdirAll', path);
      checkParents('mkdirAll', rel, true);
      if (files.has(rel)) throw fsError('EEXIST', 'mkdirAll', rel);
      mkdirs('mkdirAll', rel, rel);
    },
  };
}

/** mock の program が返す物。exitCode の既定は 0、stdout・stderr の既定は 空 (string は UTF-8)。error なら reject する。 */
export type MockProgramResult =
  | { exitCode?: number; stdout?: Uint8Array | string; stderr?: Uint8Array | string }
  | { error: string };

/** 起こされた cmd を受け 結果を決める関数。Promise を返すと 決まるまで待つ (その間の abort で reject)。 */
export type MockProgram = (cmd: ProcCmd) => MockProgramResult | Promise<MockProgramResult>;

/** 起こされた cmd の記録。stdin は byte 列にして残す。 */
export interface MockProcCall {
  name: string;
  args: string[];
  dir?: string;
  env?: Record<string, string>;
  unset?: string[];
  stdin?: Uint8Array;
}

export interface MockProc extends Proc {
  /** 起こされた cmd (起こされた順。宣言に無い名前も残る)。 */
  readonly calls: readonly MockProcCall[];
}

/** programs に宣言した名前だけを起こす Proc。親の環境変数と実の process を使わない。 */
export function createMockProc(programs: Readonly<Record<string, MockProgram>> = {}): MockProc {
  const table = new Map(Object.entries(programs));
  const calls: MockProcCall[] = [];
  return {
    calls,
    run(cmd) {
      const call: MockProcCall = { name: cmd.name, args: [...cmd.args] };
      if (cmd.dir !== undefined) call.dir = cmd.dir;
      if (cmd.env !== undefined) call.env = { ...cmd.env };
      if (cmd.unset !== undefined) call.unset = [...cmd.unset];
      if (cmd.stdin !== undefined) call.stdin = toBytes(cmd.stdin);
      calls.push(call);
      const program = table.get(cmd.name);
      if (program === undefined) return Promise.reject(undeclaredProgram(cmd.name));
      const signal = cmd.signal;
      if (signal?.aborted) return Promise.reject(abortError(cmd.name));
      return new Promise<ProcResult>((resolvePromise, rejectPromise) => {
        let settled = false;
        const onAbort = (): void => settle(() => rejectPromise(abortError(cmd.name)));
        const settle = (fn: () => void): void => {
          if (settled) return;
          settled = true;
          signal?.removeEventListener('abort', onAbort);
          fn();
        };
        signal?.addEventListener('abort', onAbort, { once: true });
        Promise.resolve()
          .then(() => program(cmd))
          .then(
            (result) =>
              settle(() => {
                if ('error' in result) {
                  rejectPromise(new Error(result.error));
                  return;
                }
                resolvePromise({
                  exitCode: result.exitCode ?? 0,
                  stdout: toBytes(result.stdout ?? ''),
                  stderr: toBytes(result.stderr ?? ''),
                });
              }),
            (err: unknown) => settle(() => rejectPromise(err)),
          );
      });
    },
  };
}

export interface MockSet extends OutsideSet {
  clock: MockClock;
  http: MockHttp;
  storage: MockStorage;
  config: MockConfig;
  idGen: MockIdGen;
  random: MockRandom;
  fs: MockFS;
  proc: MockProc;
}

export interface MockSetOptions {
  /** Clock の開始の時刻 (epoch からの ms)。 */
  startMs: number;
  routes?: readonly MockHttpRoute[];
  config?: Readonly<Record<string, string>>;
  /** Random の種。渡さなければ DEFAULT_MOCK_RANDOM_SEED。 */
  seed?: Uint8Array | string;
  /** FS の木。渡さなければ 空の根。 */
  tree?: MockFsTree;
  /** Proc が起こせる program。渡さなければ 全ての名前が 宣言に無い誤り。 */
  programs?: Readonly<Record<string, MockProgram>>;
}

/** mock の port の組を作る。 */
export function createMockSet(options: MockSetOptions): MockSet {
  return {
    clock: createMockClock(options.startMs),
    http: createMockHttp(options.routes),
    storage: createMockStorage(),
    config: createMockConfig(options.config),
    idGen: createMockIdGen(),
    random: createMockRandom(options.seed ?? DEFAULT_MOCK_RANDOM_SEED),
    fs: createMockFS(options.tree),
    proc: createMockProc(options.programs),
  };
}
