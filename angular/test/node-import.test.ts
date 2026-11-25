// Node が .ts を直に読む形 (型の除去、Node 24 以降の既定) で 部品の入口が読める事を確かめる。
// 取り込んだ project が bundler を通さずに node で読むため (値の相対 import に拡張子が無いと ERR_MODULE_NOT_FOUND)。

import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const src = (rel: string): string => fileURLToPath(new URL(`../src/${rel}`, import.meta.url));

describe('Node の型の除去で 入口を直に import できる', () => {
  for (const entry of ['index.ts', 'mock/index.ts']) {
    it(entry, () => {
      const url = new URL(`file://${src(entry)}`).href;
      const r = spawnSync(process.execPath, ['--input-type=module', '-e', `await import(${JSON.stringify(url)});`], {
        encoding: 'utf8',
      });
      expect(r.stderr).toBe('');
      expect(r.status).toBe(0);
    });
  }
});
