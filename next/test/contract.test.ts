// 契約を real と mock の両方に掛ける。
// real には test が作った物だけを渡す (機体の保存・通信・環境変数を触らない)。

import { realpathSync } from 'node:fs';
import { afterEach, vi } from 'vitest';
import {
  createRealClock,
  createRealConfig,
  createRealFS,
  createRealHttp,
  createRealProc,
  createRealIdGen,
  createRealRandom,
  createRealStorage,
  type FetchLike,
  type WebStorageLike,
} from '../src/index';
import {
  createMockClock,
  createMockConfig,
  createMockFS,
  createMockHttp,
  createMockProc,
  createMockIdGen,
  createMockRandom,
  createMockStorage,
  type MockHttpRoute,
} from '../src/mock/index';
import {
  clockContract,
  configContract,
  fsContract,
  httpContract,
  idGenContract,
  procContract,
  randomContract,
  storageContract,
  type ContractRoute,
} from './contract';
import { mockBehavior, writeScripts } from './programs';
import { testDir, writeTree } from './workdir';

// real の Clock: 後から掛けた長さ 0 の timer が呼ばれるまで待つ (event loop 1 周。時間の長さに依らない)。
clockContract('real', () => {
  const clock = createRealClock();
  return { clock, pass: () => new Promise<void>((done) => clock.setTimeout(done, 0)) };
});

clockContract('mock', () => {
  const clock = createMockClock(1_700_000_000_000);
  return {
    clock,
    pass: async () => {
      clock.advance(0);
    },
  };
});

// 契約の route から test 用の fetch を作る。通信はしない。
function fetchFromRoutes(routes: ContractRoute[]): FetchLike {
  return async (url, init) => {
    const route = routes.find((r) => r.method === init.method && r.url === url);
    if (route === undefined) throw new Error(`test fetch: no route for ${init.method} ${url}`);
    if ('error' in route) throw new Error(route.error);
    return new Response(route.body === '' ? null : route.body, {
      status: route.status,
      headers: route.headers,
    });
  };
}

httpContract('real', (routes) => createRealHttp(fetchFromRoutes(routes)));

httpContract('mock', (routes) =>
  createMockHttp(
    routes.map((r): MockHttpRoute =>
      'error' in r
        ? r
        : {
            method: r.method,
            url: r.url,
            response: { status: r.status, headers: r.headers, body: r.body },
          },
    ),
  ),
);

// test が作る Web Storage の形の object。機体の localStorage は使わない。
function memoryWebStorage(): WebStorageLike {
  const table = new Map<string, string>();
  return {
    getItem: (key) => table.get(key) ?? null,
    setItem: (key, value) => {
      table.set(key, value);
    },
    removeItem: (key) => {
      table.delete(key);
    },
  };
}

storageContract('real', () => createRealStorage(memoryWebStorage()));
storageContract('mock', () => createMockStorage());

// real の Config: 環境変数の表を test が渡す (機体の環境変数を読まない)。
configContract('real (env の表)', (table) => createRealConfig({ env: table }));
configContract('real (values の表)', (table) => createRealConfig({ values: table, env: {} }));
configContract('mock', (table) => createMockConfig(table));

// real の IdGen: 実行環境の crypto.randomUUID を使う (乱数は 外から何も読まない)。
idGenContract('real', () => createRealIdGen());
idGenContract('mock', () => createMockIdGen());

// real の Random: 実行環境の crypto.getRandomValues を使う (乱数は 外から何も読まない)。
randomContract('real', () => createRealRandom());
randomContract('mock', () => createMockRandom('contract'));

// real の FS: 根は test が .work/ の下に作った dir だけ。木は test が disk に書いてから渡す。
fsContract('real', (tree) => {
  const root = testDir();
  writeTree(root, tree);
  return createRealFS(root);
});
fsContract('mock', (tree) => createMockFS(tree));

// real の Proc: test が書いた node の script を process.execPath で起こす。親の環境変数は test が置いた 1 つだけを見る。
const PARENT_ENV = { name: 'OUTSIDE_TEST_PARENT_ENV', value: 'from-parent' };
afterEach(() => {
  vi.unstubAllEnvs();
});
procContract('real', () => {
  vi.stubEnv(PARENT_ENV.name, PARENT_ENV.value);
  const dir = realpathSync(testDir());
  const scripts = writeScripts(dir);
  return {
    proc: createRealProc({ node: process.execPath }),
    name: 'node',
    select: (behavior) => [scripts[behavior]],
    dir,
    parentEnv: PARENT_ENV,
  };
});
procContract('mock', () => ({
  proc: createMockProc({ node: mockBehavior({ [PARENT_ENV.name]: PARENT_ENV.value }) }),
  name: 'node',
  select: (behavior) => [behavior],
  dir: 'work/dir',
  parentEnv: PARENT_ENV,
}));
