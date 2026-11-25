// real の FS・Proc の test が使う dir。この repo の .work/ の下に作り 終わりに消す
// (system の一時置き場を使わない。.work/ は .gitignore 済み。go の test と同じ置き場)。

import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterAll } from 'vitest';

const base = fileURLToPath(new URL('../../.work/', import.meta.url));
const made: string[] = [];

afterAll(() => {
  for (const dir of made.splice(0)) rmSync(dir, { recursive: true, force: true });
});

/** .work/ の下に 新しい空の dir を作り その絶対 path を返す。test file の終わりに消す。 */
export function testDir(): string {
  mkdirSync(base, { recursive: true });
  const dir = mkdtempSync(join(base, 'next-outside-'));
  made.push(dir);
  return dir;
}

/** dir の下に 相対 path → 中身 の木を書く (末尾 / は 空の dir)。 */
export function writeTree(dir: string, tree: Readonly<Record<string, string | Uint8Array>>): void {
  for (const [rel, value] of Object.entries(tree)) {
    if (rel.endsWith('/')) {
      mkdirSync(join(dir, rel), { recursive: true });
      continue;
    }
    const file = join(dir, rel);
    mkdirSync(dirname(file), { recursive: true });
    writeFileSync(file, value);
  }
}
