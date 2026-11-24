// TC-OUT-LINT-TS: 設定は 外を直に呼ぶ code を落とし 口を通す code を通す。
import { test } from "node:test";
import assert from "node:assert/strict";
import { ESLint } from "eslint";

const here = new URL(".", import.meta.url).pathname;
const lint = (dir) => new ESLint({ cwd: here, overrideConfigFile: "eslint.config.js" }).lintFiles([`testdata/${dir}/*.js`]);

test("外を直に呼ぶ code を落とす (禁じた 11 の呼び出しが全部 誤り)", async () => {
  const [r] = await lint("bad");
  const rules = r.messages.map((m) => m.ruleId);
  assert.equal(r.errorCount, 11, JSON.stringify(r.messages.map((m) => `${m.line}:${m.ruleId}`)));
  for (const id of ["no-restricted-syntax", "no-restricted-globals", "no-restricted-properties", "no-restricted-imports"]) {
    assert.ok(rules.includes(id), `${id} が出ていない`);
  }
});

test("口を通す code と 決めた時刻の Date は通す", async () => {
  const [r] = await lint("good");
  assert.equal(r.errorCount, 0, JSON.stringify(r.messages));
});
