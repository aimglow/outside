// Angular 向けの薄い口。port を DI で受け取るための token と provider だけを持つ。

import { InjectionToken, type Provider } from '@angular/core';
import type { Clock, Config, Http, OutsideSet, Storage } from '../index';

export const OUTSIDE_CLOCK = new InjectionToken<Clock>('OUTSIDE_CLOCK');
export const OUTSIDE_HTTP = new InjectionToken<Http>('OUTSIDE_HTTP');
export const OUTSIDE_STORAGE = new InjectionToken<Storage>('OUTSIDE_STORAGE');
export const OUTSIDE_CONFIG = new InjectionToken<Config>('OUTSIDE_CONFIG');

/** 渡した set (real でも fake でも) の port を token に結ぶ。 */
export function provideOutside(set: OutsideSet): Provider[] {
  return [
    { provide: OUTSIDE_CLOCK, useValue: set.clock },
    { provide: OUTSIDE_HTTP, useValue: set.http },
    { provide: OUTSIDE_STORAGE, useValue: set.storage },
    { provide: OUTSIDE_CONFIG, useValue: set.config },
  ];
}
