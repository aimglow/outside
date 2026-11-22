// port と method の名前の組を確かめる。
// 期待の表は 設計書の ports.frontend_and_mobile を写した物 (設計書の file は test から読まない)。

import { describe, expect, it } from 'vitest';
import { createRealSet } from '../src/index';
import { createMockSet } from '../src/mock/index';

const DESIGN_PORTS: Record<string, string[]> = {
  clock: ['now', 'setTimeout', 'clearTimeout'],
  http: ['request'],
  storage: ['get', 'set', 'remove'],
  config: ['get'],
};

// port が持つ関数の名前の一覧。
function methodNames(port: object): string[] {
  return Object.entries(port)
    .filter(([, value]) => typeof value === 'function')
    .map(([name]) => name)
    .sort();
}

describe('TC-OUT-PARITY port と method の名前', () => {
  it('real の set は 設計書と同じ組', () => {
    const set = createRealSet({ storage: { getItem: () => null, setItem: () => {}, removeItem: () => {} } });
    expect(Object.keys(set).sort()).toEqual(Object.keys(DESIGN_PORTS).sort());
    for (const [port, methods] of Object.entries(DESIGN_PORTS)) {
      expect(methodNames(set[port as keyof typeof set]), port).toEqual([...methods].sort());
    }
  });

  it('mock の set は 設計書の method を全部持つ', () => {
    const set = createMockSet({ startMs: 0 });
    expect(Object.keys(set).sort()).toEqual(Object.keys(DESIGN_PORTS).sort());
    for (const [port, methods] of Object.entries(DESIGN_PORTS)) {
      expect(methodNames(set[port as keyof typeof set]), port).toEqual(expect.arrayContaining(methods));
    }
  });
});
