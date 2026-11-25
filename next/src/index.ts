// outside の口 (port) と 本物の実装 (real)。Node と Next.js 向け (部品 next)。
// 外 (時計・timer・通信・browser の保存・環境変数・乱数・file・子 process) を直に触るのは この file だけ。
// 依存は node:* の標準 module だけ (next・react・@angular/* を使わない)。
// 外の物は 使う時に初めて取りに行く (import しただけでは 何も触らない)。node:* も 使う時に 動的な import で取る
// (browser で この file を import しても node:* を読みに行かない)。

import {
  NO_ROOT,
  OUTSIDE_ROOT,
  abortError,
  baseName,
  byName,
  cleanRel,
  fsError,
  outsideError,
  rootRemoval,
  toBytes,
  undeclaredProgram,
} from './rule';
import type { SpawnOptions } from 'node:child_process';

export { NO_ROOT, OUTSIDE_ROOT, UNDECLARED_PROGRAM, type OutsideError } from './rule';

/** timer の控え。clearTimeout に渡すためだけに使う。 */
export type TimerHandle = unknown;

/** 今の時刻を返す。timer を掛ける。 */
export interface Clock {
  /** 今の時刻 (epoch からの ms)。 */
  now(): number;
  /** ms 後に fn を 1 回呼ぶ。同期では呼ばない。 */
  setTimeout(fn: () => void, ms: number): TimerHandle;
  /** 掛けた timer を取り消す。済んだ timer や知らない控えは 何もしない。 */
  clearTimeout(handle: TimerHandle): void;
}

export interface HttpRequest {
  method: string;
  url: string;
  headers?: Record<string, string>;
  body?: string;
}

export interface HttpResponse {
  status: number;
  /** 名前は小文字。 */
  headers: Record<string, string>;
  body: string;
}

/** 通信を送り 応答を返す。 */
export interface Http {
  /** 応答が届けば status に依らず resolve する。届かなければ reject する。 */
  request(req: HttpRequest): Promise<HttpResponse>;
}

/** browser か端末の保存を読む・書く。 */
export interface Storage {
  /** 無い key は null。 */
  get(key: string): string | null;
  set(key: string, value: string): void;
  remove(key: string): void;
}

/** build 時と実行時の設定の値を返す。 */
export interface Config {
  /** 無い名前は undefined。 */
  get(name: string): string | undefined;
}

/** 新しい ID を返す。 */
export interface IdGen {
  /** 呼ぶたびに 違う ID を返す。空の文字列は返さない。 */
  newId(): string;
}

/** 長さを決めた乱数の byte 列を返す (trace の id・nonce 等)。 */
export interface Random {
  /** buf を 全部 乱数の byte で埋め 同じ buf を返す。 */
  read(buf: Uint8Array): Uint8Array;
}

/** readDir の 1 件。 */
export interface FsDirEntry {
  name: string;
  isDir: boolean;
}

/** stat の結果。dir の size は 0。 */
export interface FsFileInfo {
  name: string;
  size: number;
  isDir: boolean;
}

/**
 * 宣言した根の下の file を読む・書く・一覧する (Node だけ)。
 * path は 根からの相対 (区切りは /、根そのものは '.')。絶対 path と 根の外へ出る path は code = 'EOUTSIDEROOT' で reject。
 * 無い path は code = 'ENOENT' で reject。他の誤りも node の誤りと同じ code (EISDIR・ENOTDIR・ENOTEMPTY・EEXIST) を持つ。
 */
export interface FS {
  readFile(path: string): Promise<Uint8Array>;
  /** string は UTF-8 で書く。親の dir が無ければ作る。 */
  writeFile(path: string, data: Uint8Array | string): Promise<void>;
  /** 名前の順。 */
  readDir(path: string): Promise<FsDirEntry[]>;
  stat(path: string): Promise<FsFileInfo>;
  /** file か 空の dir を消す。中身の在る dir は ENOTEMPTY、根は EPERM。 */
  remove(path: string): Promise<void>;
  /** 入れ子の dir を作る。在れば何もしない。file が在れば EEXIST。 */
  mkdirAll(path: string): Promise<void>;
}

/** Proc.run に渡す 起こす program と その入力。 */
export interface ProcCmd {
  /** 宣言した program の名前。 */
  name: string;
  args: readonly string[];
  /** 子の作業 dir。無ければ 親と同じ。 */
  dir?: string;
  /** 子へ足す環境変数 (unset より後に当てる = 同じ名前は env が効く)。 */
  env?: Readonly<Record<string, string>>;
  /** 子へ渡さない 親の環境変数の名前。 */
  unset?: readonly string[];
  /** 子の標準入力 (string は UTF-8)。無ければ 空の入力。 */
  stdin?: Uint8Array | string;
  /** abort したら 子を止めて reject する。 */
  signal?: AbortSignal;
}

export interface ProcResult {
  exitCode: number;
  stdout: Uint8Array;
  stderr: Uint8Array;
}

/** 宣言した名前の program を起こし 結果を返す (Node だけ)。shell を通さない。 */
export interface Proc {
  /**
   * 0 以外の exitCode も resolve する。宣言に無い name は code = 'EUNDECLARED'、
   * abort は name = 'AbortError' (code = 'ABORT_ERR')、起こせない program は code = 'ENOENT' 等で reject。
   */
  run(cmd: ProcCmd): Promise<ProcResult>;
}

/** 1 つの処理に渡す port の組。 */
export interface OutsideSet {
  clock: Clock;
  http: Http;
  storage: Storage;
  config: Config;
  idGen: IdGen;
  random: Random;
  fs: FS;
  proc: Proc;
}

/** real が使う fetch の形。 */
export type FetchLike = (
  url: string,
  init: { method: string; headers?: Record<string, string>; body?: string },
) => Promise<{
  status: number;
  headers: { forEach(fn: (value: string, name: string) => void): void };
  text(): Promise<string>;
}>;

/** real が使う Web Storage の形 (localStorage・sessionStorage が満たす)。 */
export interface WebStorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

/** 設定の値の表。 */
export type ConfigTable = Readonly<Record<string, string | undefined>>;

// 実行環境の global から 名前で 1 つ取る。無ければ 何が無いかを名乗って throw する。
function needGlobal<T>(name: string): T {
  const value = (globalThis as Record<string, unknown>)[name];
  if (value === undefined || value === null) {
    throw new Error(`outside: ${name} is not available in this environment`);
  }
  return value as T;
}

/** 実の時計と実の timer を使う Clock。 */
export function createRealClock(): Clock {
  return {
    now: () => Date.now(),
    setTimeout: (fn, ms) =>
      needGlobal<(fn: () => void, ms: number) => unknown>('setTimeout')(fn, ms),
    clearTimeout: (handle) => {
      needGlobal<(handle: unknown) => void>('clearTimeout')(handle);
    },
  };
}

/** fetch で通信する Http。fetchFn を渡さなければ 実行環境の fetch を使う。 */
export function createRealHttp(fetchFn?: FetchLike): Http {
  return {
    async request(req) {
      const doFetch = fetchFn ?? needGlobal<FetchLike>('fetch');
      const init: Parameters<FetchLike>[1] = { method: req.method };
      if (req.headers !== undefined) init.headers = req.headers;
      if (req.body !== undefined) init.body = req.body;
      // global の fetch は this が外れると動かない実装があるため 関数として呼ぶ。
      const res = await doFetch(req.url, init);
      const headers: Record<string, string> = {};
      res.headers.forEach((value, name) => {
        headers[name.toLowerCase()] = value;
      });
      return { status: res.status, headers, body: await res.text() };
    },
  };
}

/**
 * Web Storage に保存する Storage。
 * area に 'local' か 'session' を渡すと 実行環境の localStorage・sessionStorage を使う。
 * Web Storage の形の object を渡すと それを使う。
 */
export function createRealStorage(
  area: 'local' | 'session' | WebStorageLike = 'local',
): Storage {
  const resolve = (): WebStorageLike => {
    if (area === 'local') return needGlobal<WebStorageLike>('localStorage');
    if (area === 'session') return needGlobal<WebStorageLike>('sessionStorage');
    return area;
  };
  return {
    get: (key) => resolve().getItem(key),
    set: (key, value) => {
      resolve().setItem(key, value);
    },
    remove: (key) => {
      resolve().removeItem(key);
    },
  };
}

export interface RealConfigOptions {
  /** application の入口が渡す値の表 (build 時に埋め込まれた値・実行時に受け取った値)。env より先に引く。 */
  values?: ConfigTable;
  /** 環境変数の表。渡さなければ process.env を使う (process が無い環境では 空の表)。 */
  env?: ConfigTable;
}

/** values → 環境変数 の順に引く Config。 */
export function createRealConfig(options: RealConfigOptions = {}): Config {
  const resolveEnv = (): ConfigTable => {
    if (options.env !== undefined) return options.env;
    const proc = (globalThis as { process?: { env?: ConfigTable } }).process;
    return proc?.env ?? {};
  };
  // 表が持つ文字列の値だけを返す (object が元から持つ名前を 値として返さない)。
  const lookup = (table: ConfigTable, name: string): string | undefined => {
    if (!Object.prototype.hasOwnProperty.call(table, name)) return undefined;
    const value = table[name];
    return typeof value === 'string' ? value : undefined;
  };
  return {
    get(name) {
      const fromValues =
        options.values === undefined ? undefined : lookup(options.values, name);
      return fromValues ?? lookup(resolveEnv(), name);
    },
  };
}

/** crypto.randomUUID で UUID v4 を返す IdGen。crypto は 使う時に取る。 */
export function createRealIdGen(): IdGen {
  return {
    newId() {
      const crypto = needGlobal<{ randomUUID?: () => string }>('crypto');
      if (typeof crypto.randomUUID !== 'function') {
        throw new Error('outside: crypto.randomUUID is not available in this environment');
      }
      // this が外れると動かない実装があるため crypto の method として呼ぶ。
      return crypto.randomUUID();
    },
  };
}

/** crypto.getRandomValues が 1 回に埋められる byte の上限。 */
const GET_RANDOM_VALUES_MAX = 65536;

/** crypto.getRandomValues で埋める Random。crypto は 使う時に取る。上限より長い buf は 分けて埋める。 */
export function createRealRandom(): Random {
  return {
    read(buf) {
      const crypto = needGlobal<{ getRandomValues?: (array: Uint8Array) => Uint8Array }>('crypto');
      if (typeof crypto.getRandomValues !== 'function') {
        throw new Error('outside: crypto.getRandomValues is not available in this environment');
      }
      for (let start = 0; start < buf.length; start += GET_RANDOM_VALUES_MAX) {
        // this が外れると動かない実装があるため crypto の method として呼ぶ。
        crypto.getRandomValues(buf.subarray(start, start + GET_RANDOM_VALUES_MAX));
      }
      return buf;
    },
  };
}

type NodeFs = typeof import('node:fs/promises');
type NodePath = typeof import('node:path');
type NodeChildProcess = typeof import('node:child_process');

// node:* を 使う時に取る。取れない環境 (browser) では 何が無いかを名乗って reject する。
async function loadNode<T>(name: string, load: () => Promise<T>): Promise<T> {
  try {
    return await load();
  } catch {
    throw new Error(`outside: ${name} is not available in this environment`);
  }
}
const loadFs = (): Promise<NodeFs> => loadNode('node:fs/promises', () => import('node:fs/promises'));
const loadPath = (): Promise<NodePath> => loadNode('node:path', () => import('node:path'));
const loadChildProcess = (): Promise<NodeChildProcess> =>
  loadNode('node:child_process', () => import('node:child_process'));

// node の誤りを 相対 path で名乗る誤りにする (code は そのまま)。code の無い誤りは そのまま投げる。
function fromNodeError(err: unknown, op: string, rel: string): unknown {
  const code = (err as { code?: unknown } | null)?.code;
  if (typeof code === 'string' && !(err as Error).message.startsWith('outside')) {
    return fsError(code, op, rel);
  }
  return err;
}

const isAbsolutePath = (p: string): boolean =>
  p.startsWith('/') || p.startsWith('\\') || /^[A-Za-z]:[\\/]/.test(p);

/** symlink を辿る回数の上限 (Linux の MAXSYMLINKS と同じ)。 */
const MAX_SYMLINK_HOPS = 40;

/**
 * root (絶対 path) を根とする FS。使う時に node:fs/promises を取る。
 * path は 1 つずつ lstat で辿り symlink は 中身を読んで 根の中に解ける物だけを辿る
 * (根の外へ出る symlink・根の外を指す 切れた symlink も 誤り)。
 */
export function createRealFS(root: string): FS {
  if (!isAbsolutePath(root)) {
    throw new Error(`outside FS: the root must be an absolute path: ${root}`);
  }

  // 根の実の path を引き 根の中の相対 path を 実の path に解く。followLast が false なら 最後の名前の symlink は辿らない。
  const resolve = async (
    op: string,
    rel: string,
    followLast: boolean,
  ): Promise<{ fsm: NodeFs; pathm: NodePath; abs: string }> => {
    const fsm = await loadFs();
    const pathm = await loadPath();
    let rootReal: string;
    try {
      rootReal = await fsm.realpath(root);
    } catch (err) {
      throw fromNodeError(err, op, '. (root)');
    }
    const rootGiven = pathm.resolve(root);
    const prefixOf = (dir: string): string => (dir.endsWith(pathm.sep) ? dir : dir + pathm.sep);
    const within = (dir: string, p: string): boolean => p === dir || p.startsWith(prefixOf(dir));
    const outside = (): Error => outsideError(OUTSIDE_ROOT, `outside FS: ${op} ${rel}: path leaves the root through a symlink`);
    const splitTarget = (t: string): string[] => t.split(pathm.sep === '\\' ? /[\\/]/ : '/');

    let cur = rootReal;
    const queue = rel === '.' ? [] : rel.split('/');
    let hops = 0;
    while (queue.length > 0) {
      const seg = queue.shift() as string;
      if (seg === '' || seg === '.') continue;
      if (seg === '..') {
        cur = pathm.dirname(cur);
        if (!within(rootReal, cur)) throw outside();
        continue;
      }
      const next = pathm.join(cur, seg);
      const isLast = queue.length === 0;
      let isLink: boolean;
      try {
        isLink = (await fsm.lstat(next)).isSymbolicLink();
      } catch (err) {
        if ((err as { code?: unknown }).code === 'ENOENT') {
          // ここから先は無い。無い dir を .. で戻る形は 実の file system では解けない (無い path)。
          if (queue.includes('..')) throw fsError('ENOENT', op, rel);
          return { fsm, pathm, abs: pathm.join(next, ...queue) };
        }
        throw fromNodeError(err, op, rel);
      }
      if (!isLink || (isLast && !followLast)) {
        cur = next;
        continue;
      }
      hops += 1;
      if (hops > MAX_SYMLINK_HOPS) throw fsError('ELOOP', op, rel);
      const target = await fsm.readlink(next);
      if (isAbsolutePath(target)) {
        const t = pathm.resolve(target);
        const base = within(rootReal, t) ? rootReal : within(rootGiven, t) ? rootGiven : undefined;
        if (base === undefined) throw outside();
        cur = rootReal;
        queue.unshift(...splitTarget(pathm.relative(base, t)));
      } else {
        // 相対の target は link の在る dir (cur) から解く。
        queue.unshift(...splitTarget(target));
      }
    }
    return { fsm, pathm, abs: cur };
  };

  return {
    async readFile(path) {
      const rel = cleanRel(path);
      const { fsm, abs } = await resolve('readFile', rel, true);
      try {
        return new Uint8Array(await fsm.readFile(abs));
      } catch (err) {
        throw fromNodeError(err, 'readFile', rel);
      }
    },
    async writeFile(path, data) {
      const rel = cleanRel(path);
      if (rel === '.') throw fsError('EISDIR', 'writeFile', rel);
      const { fsm, pathm, abs } = await resolve('writeFile', rel, true);
      try {
        await fsm.mkdir(pathm.dirname(abs), { recursive: true });
        await fsm.writeFile(abs, toBytes(data));
      } catch (err) {
        throw fromNodeError(err, 'writeFile', rel);
      }
    },
    async readDir(path) {
      const rel = cleanRel(path);
      const { fsm, abs } = await resolve('readDir', rel, true);
      try {
        const entries = await fsm.readdir(abs, { withFileTypes: true });
        return entries.map((e) => ({ name: e.name, isDir: e.isDirectory() })).sort(byName);
      } catch (err) {
        throw fromNodeError(err, 'readDir', rel);
      }
    },
    async stat(path) {
      const rel = cleanRel(path);
      const { fsm, abs } = await resolve('stat', rel, true);
      try {
        const st = await fsm.stat(abs);
        const isDir = st.isDirectory();
        return { name: baseName(rel), size: isDir ? 0 : st.size, isDir };
      } catch (err) {
        throw fromNodeError(err, 'stat', rel);
      }
    },
    async remove(path) {
      const rel = cleanRel(path);
      if (rel === '.') throw rootRemoval();
      // 最後の名前の symlink は辿らない (link そのものを消す)。
      const { fsm, abs } = await resolve('remove', rel, false);
      try {
        const st = await fsm.lstat(abs);
        if (st.isDirectory()) await fsm.rmdir(abs);
        else await fsm.unlink(abs);
      } catch (err) {
        // 中身の在る dir の rmdir は OS により EEXIST を返す。ENOTEMPTY に揃える。
        if ((err as { code?: unknown }).code === 'EEXIST') throw fsError('ENOTEMPTY', 'remove', rel);
        throw fromNodeError(err, 'remove', rel);
      }
    },
    async mkdirAll(path) {
      const rel = cleanRel(path);
      const { fsm, abs } = await resolve('mkdirAll', rel, true);
      try {
        const st = await fsm.stat(abs).catch(() => undefined);
        if (st !== undefined) {
          if (st.isDirectory()) return;
          throw fsError('EEXIST', 'mkdirAll', rel);
        }
        await fsm.mkdir(abs, { recursive: true });
      } catch (err) {
        throw fromNodeError(err, 'mkdirAll', rel);
      }
    },
  };
}

/** 根が宣言されていない FS。どの method も 根が宣言されていないと名乗って reject する。 */
function createUnrootedFS(): FS {
  const fail = (): Promise<never> =>
    Promise.reject(outsideError(NO_ROOT, 'outside FS: the root is not declared (pass fsRoot to createRealSet)'));
  return {
    readFile: fail,
    writeFile: fail,
    readDir: fail,
    stat: fail,
    remove: fail,
    mkdirAll: fail,
  };
}

function concatBytes(chunks: readonly Uint8Array[]): Uint8Array {
  let length = 0;
  for (const c of chunks) length += c.length;
  const out = new Uint8Array(length);
  let at = 0;
  for (const c of chunks) {
    out.set(c, at);
    at += c.length;
  }
  return out;
}

/**
 * programs (名前 → 実行 file の path) に在る program だけを起こす Proc。path が空の名前は 子の環境変数の PATH から探す。
 * node:child_process の spawn で shell を通さずに起こす。子の環境変数 = process.env から unset を外し env を足した物。
 */
export function createRealProc(programs: Readonly<Record<string, string>> = {}): Proc {
  const table = new Map(Object.entries(programs));
  return {
    async run(cmd) {
      const file = table.get(cmd.name);
      if (file === undefined) throw undeclaredProgram(cmd.name);
      const cp = await loadChildProcess();
      const parent = needGlobal<{ env?: Record<string, string | undefined> }>('process');
      const env: Record<string, string | undefined> = { ...(parent.env ?? {}) };
      for (const name of cmd.unset ?? []) delete env[name];
      Object.assign(env, cmd.env ?? {});
      const signal = cmd.signal;
      if (signal?.aborted) throw abortError(cmd.name);

      return new Promise<ProcResult>((resolvePromise, rejectPromise) => {
        let settled = false;
        const stdout: Uint8Array[] = [];
        const stderr: Uint8Array[] = [];
        const onAbort = (): void => {
          child.kill('SIGKILL');
          settle(() => rejectPromise(abortError(cmd.name)));
        };
        const settle = (fn: () => void): void => {
          if (settled) return;
          settled = true;
          signal?.removeEventListener('abort', onAbort);
          fn();
        };
        const options: SpawnOptions = {
          env,
          stdio: [cmd.stdin === undefined ? 'ignore' : 'pipe', 'pipe', 'pipe'],
          shell: false,
          windowsHide: true,
        };
        if (cmd.dir !== undefined) options.cwd = cmd.dir;
        const child = cp.spawn(file === '' ? cmd.name : file, [...cmd.args], options);
        child.on('error', (err: Error & { code?: string }) => {
          settle(() =>
            rejectPromise(
              outsideError(
                err.code ?? 'ESPAWN',
                `outside Proc: program ${cmd.name} (${file === '' ? 'from PATH' : file}) could not be run: ${err.message}`,
              ),
            ),
          );
        });
        child.stdout?.on('data', (chunk: Uint8Array) => stdout.push(chunk));
        child.stderr?.on('data', (chunk: Uint8Array) => stderr.push(chunk));
        child.on('close', (code: number | null, sig: string | null) => {
          settle(() => {
            if (code === null) {
              rejectPromise(
                outsideError('ESIGNAL', `outside Proc: program ${cmd.name} was terminated by signal ${sig ?? 'unknown'}`),
              );
              return;
            }
            resolvePromise({ exitCode: code, stdout: concatBytes(stdout), stderr: concatBytes(stderr) });
          });
        });
        signal?.addEventListener('abort', onAbort, { once: true });
        if (cmd.stdin !== undefined && child.stdin) {
          // 子が標準入力を読まずに終えた時の EPIPE は 結果に関わらない。
          child.stdin.on('error', () => {});
          child.stdin.end(toBytes(cmd.stdin));
        }
      });
    },
  };
}

export interface RealSetOptions {
  fetch?: FetchLike;
  storage?: 'local' | 'session' | WebStorageLike;
  config?: RealConfigOptions;
  /** FS の根 (絶対 path)。無ければ fs は 使った時に 根が宣言されていないと名乗って reject する。 */
  fsRoot?: string;
  /** Proc が起こしてよい program (名前 → 実行 file の path、空なら PATH から)。無ければ 全ての名前が 宣言に無い誤り。 */
  programs?: Readonly<Record<string, string>>;
}

/** real の port の組を作る。 */
export function createRealSet(options: RealSetOptions = {}): OutsideSet {
  return {
    clock: createRealClock(),
    http: createRealHttp(options.fetch),
    storage: createRealStorage(options.storage),
    config: createRealConfig(options.config),
    idGen: createRealIdGen(),
    random: createRealRandom(),
    fs: options.fsRoot === undefined ? createUnrootedFS() : createRealFS(options.fsRoot),
    proc: createRealProc(options.programs ?? {}),
  };
}
