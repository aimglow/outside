// Angular の口: provideOutside で結んだ port が token で引ける。

import { Injector } from '@angular/core';
import { describe, expect, it } from 'vitest';
import {
  OUTSIDE_CLOCK,
  OUTSIDE_CONFIG,
  OUTSIDE_HTTP,
  OUTSIDE_STORAGE,
  provideOutside,
} from '../src/angular/index';
import { createMockSet } from '../src/mock/index';

describe('REQ-OUT-INJECT Angular の口', () => {
  it('provideOutside で渡した set の port が token で引ける', () => {
    const set = createMockSet({ startMs: 0 });
    const injector = Injector.create({ providers: provideOutside(set) });
    expect(injector.get(OUTSIDE_CLOCK)).toBe(set.clock);
    expect(injector.get(OUTSIDE_HTTP)).toBe(set.http);
    expect(injector.get(OUTSIDE_STORAGE)).toBe(set.storage);
    expect(injector.get(OUTSIDE_CONFIG)).toBe(set.config);
  });
});
