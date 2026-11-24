// TC-OUT-RUNTIME-DEPS: 部品 next の依存の宣言と src の import が runtime の標準の外を持たない。
// package.json と src の source を そのまま読んで確かめる (この部品の中の file だけを読む)。

import { readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const root = fileURLToPath(new URL('../', import.meta.url));

// source の文から import・export from・動的 import・require の指定子を全部取り出す。
function importSpecifiers(source: string): string[] {
  const found: string[] = [];
  const patterns = [
    /\b(?:import|export)\s[^'"]*?\bfrom\s*['"]([^'"]+)['"]/g,
    /\bimport\s*['"]([^'"]+)['"]/g,
    /\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)/g,
    /\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)/g,
  ];
  for (const pattern of patterns) {
    for (const match of source.matchAll(pattern)) {
      if (match[1] !== undefined) found.push(match[1]);
    }
  }
  return found;
}

const isRelative = (spec: string): boolean => spec.startsWith('./') || spec.startsWith('../');
// この部品の src が import してよい指定子 = node:* と 相対だけ。
const allowed: (spec: string) => boolean = (spec) => isRelative(spec) || spec.startsWith('node:');

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { recursive: true, encoding: 'utf8' })
    .filter((name) => name.endsWith('.ts'))
    .map((name) => `${dir}/${name}`)
    .sort();
}

describe('TC-OUT-RUNTIME-DEPS 部品 next の依存', () => {
  it('取り出しの陽性対照: 禁じた指定子を 書き方に依らず拾う', () => {
    const sample = [
      "import { InjectionToken } from '@angular/core';",
      "import type { X } from 'react';",
      "export { y } from 'next/server';",
      "import 'zone.js';",
      "const m = await import('memfs');",
      "const f = require('fs');",
      "import { a } from './a';",
    ].join('\n');
    const specs = importSpecifiers(sample);
    expect(specs.sort()).toEqual(['./a', '@angular/core', 'fs', 'memfs', 'next/server', 'react', 'zone.js']);
    expect(specs.filter((spec) => !allowed(spec)).sort()).toEqual(
      ['@angular/core', 'fs', 'memfs', 'next/server', 'react', 'zone.js'],
    );
    expect(['./a', '../b', 'node:fs'].every(allowed)).toBe(true);
  });

  it('package.json の dependencies は空。どの依存の表にも @angular/*・next・react が無い', () => {
    const pkg = JSON.parse(readFileSync(`${root}package.json`, 'utf8')) as Record<string, unknown>;
    const deps = (pkg['dependencies'] ?? {}) as Record<string, string>;
    expect(Object.keys(deps)).toEqual([]);
    expect(pkg['peerDependencies'] ?? {}).toEqual({});
    expect(pkg['optionalDependencies'] ?? {}).toEqual({});
    const named = Object.keys((pkg['devDependencies'] ?? {}) as Record<string, string>);
    expect(named.filter((name) => /^(@angular\/|next$|react$|react-dom$)/.test(name))).toEqual([]);
  });

  it('src の import は node:* と 相対だけ', () => {
    const files = sourceFiles(`${root}src`);
    expect(files.length).toBeGreaterThan(0);
    const outside: string[] = [];
    for (const file of files) {
      for (const spec of importSpecifiers(readFileSync(file, 'utf8'))) {
        if (!allowed(spec)) outside.push(`${file.slice(root.length)}: ${spec}`);
      }
    }
    expect(outside).toEqual([]);
  });
});
