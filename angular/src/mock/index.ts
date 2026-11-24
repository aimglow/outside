// outside のmock (mock)。口 1 つにつき 1 つ。
// test が渡した値だけで振る舞いが決まる。実の時計・環境変数・通信・保存を読まない。
// 呼ばれた内容は calls から読める。

import type {
  Clock,
  Config,
  Http,
  HttpRequest,
  IdGen,
  HttpResponse,
  OutsideSet,
  Storage,
  TimerHandle,
} from '../index';

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

export interface MockSet extends OutsideSet {
  clock: MockClock;
  http: MockHttp;
  storage: MockStorage;
  config: MockConfig;
  idGen: MockIdGen;
}

export interface MockSetOptions {
  /** Clock の開始の時刻 (epoch からの ms)。 */
  startMs: number;
  routes?: readonly MockHttpRoute[];
  config?: Readonly<Record<string, string>>;
}

/** mock の port の組を作る。 */
export function createMockSet(options: MockSetOptions): MockSet {
  return {
    clock: createMockClock(options.startMs),
    http: createMockHttp(options.routes),
    storage: createMockStorage(),
    config: createMockConfig(options.config),
    idGen: createMockIdGen(),
  };
}
