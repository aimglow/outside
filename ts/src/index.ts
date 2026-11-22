// outside の口 (port) と 本物の実装 (real)。
// 外 (時計・timer・通信・browser の保存・環境変数) を直に触るのは この file だけ。
// 外の物は 使う時に初めて取りに行く (import しただけでは 何も触らない)。

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

/** 1 つの処理に渡す port の組。 */
export interface OutsideSet {
  clock: Clock;
  http: Http;
  storage: Storage;
  config: Config;
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

export interface RealSetOptions {
  fetch?: FetchLike;
  storage?: 'local' | 'session' | WebStorageLike;
  config?: RealConfigOptions;
}

/** real の port の組を作る。 */
export function createRealSet(options: RealSetOptions = {}): OutsideSet {
  return {
    clock: createRealClock(),
    http: createRealHttp(options.fetch),
    storage: createRealStorage(options.storage),
    config: createRealConfig(options.config),
  };
}
