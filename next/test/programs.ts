// 契約の program の振る舞い。real は test が作った node の script を process.execPath で起こし、
// mock は 同じ振る舞いの関数を使う (振る舞いの正本は SCRIPTS と mockBehavior の 2 つで 同じ物を書く)。

import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { ProcCmd } from '../src/index';
import type { MockProgramResult } from '../src/mock/index';
import type { ContractBehavior } from './contract';

/** 振る舞い → node の script (CommonJS)。引数は process.argv[2] 以降。 */
const SCRIPTS: Record<ContractBehavior, string> = {
  echo: "process.stdout.write(process.argv.slice(2).join(' ') + '\\n');",
  fail: "process.stderr.write('bad\\n'); process.exitCode = 3;",
  cat: 'process.stdin.pipe(process.stdout);',
  env: [
    'const out = {};',
    'for (const name of process.argv.slice(2)) out[name] = process.env[name] ?? null;',
    'process.stdout.write(JSON.stringify(out));',
  ].join('\n'),
  pwd: "process.stdout.write(process.cwd() + '\\n');",
  // 起きた印に 自分の pid を 引数の file へ書き 止められるまで待つ。
  hang: [
    "if (process.argv[2]) require('node:fs').writeFileSync(process.argv[2], String(process.pid));",
    'setInterval(() => {}, 1000);',
  ].join('\n'),
};

/** dir の下に 振る舞いごとの script を書き 振る舞い → script の絶対 path を返す。 */
export function writeScripts(dir: string): Record<ContractBehavior, string> {
  const out = {} as Record<ContractBehavior, string>;
  for (const [behavior, source] of Object.entries(SCRIPTS) as [ContractBehavior, string][]) {
    const file = join(dir, `${behavior}.cjs`);
    writeFileSync(file, `${source}\n`);
    out[behavior] = file;
  }
  return out;
}

/** mock の program: 最初の引数で振る舞いを選び script と同じ結果を返す。parent は 親の環境変数の代わりの表。 */
export function mockBehavior(parent: Readonly<Record<string, string>>) {
  return (cmd: ProcCmd): MockProgramResult | Promise<MockProgramResult> => {
    const [behavior, ...rest] = cmd.args;
    switch (behavior as ContractBehavior) {
      case 'echo':
        return { stdout: `${rest.join(' ')}\n` };
      case 'fail':
        return { exitCode: 3, stderr: 'bad\n' };
      case 'cat':
        return { stdout: cmd.stdin ?? '' };
      case 'env': {
        const env: Record<string, string> = { ...parent };
        for (const name of cmd.unset ?? []) delete env[name];
        Object.assign(env, cmd.env ?? {});
        const out: Record<string, string | null> = {};
        for (const name of rest) out[name] = env[name] ?? null;
        return { stdout: JSON.stringify(out) };
      }
      case 'pwd':
        return { stdout: `${cmd.dir ?? ''}\n` };
      case 'hang':
        return new Promise<MockProgramResult>(() => {});
      default:
        return { error: `mock program: unknown behavior ${String(behavior)}` };
    }
  };
}
