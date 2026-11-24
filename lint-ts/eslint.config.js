// outside の TypeScript の lint の設定 (設計書 DSG-outside-001 の lint.typescript)。
// 外を直に呼ぶ code を落とす。部品の real の module だけが例外 (取り込む project は real を持たない)。
// 取り込み方: project の eslint.config.js で `import outside from "<写しの path>/lint-ts/eslint.config.js"` して 配列に足す。

const port = (name) => `外を直に呼んでいる。outside の ${name} の口を通す (test では mock を渡す)`;

const restrictedSyntax = [
  { selector: "CallExpression[callee.object.name='Date'][callee.property.name='now']", message: port("Clock (now)") },
  { selector: "NewExpression[callee.name='Date'][arguments.length=0]", message: port("Clock (now)") },
  { selector: "MemberExpression[object.name='process'][property.name='env']", message: port("Config") },
  { selector: "MemberExpression[object.object.name='globalThis'][object.property.name='process'][property.name='env']", message: port("Config") },
];

const restrictedGlobals = [
  { name: "setTimeout", message: port("Clock (setTimeout)") },
  { name: "setInterval", message: port("Clock (setTimeout)") },
  { name: "fetch", message: port("Http") },
  { name: "localStorage", message: port("Storage") },
  { name: "sessionStorage", message: port("Storage") },
];

const restrictedProperties = [
  { object: "window", property: "setTimeout", message: port("Clock (setTimeout)") },
  { object: "window", property: "setInterval", message: port("Clock (setTimeout)") },
  { object: "window", property: "fetch", message: port("Http") },
  { object: "window", property: "localStorage", message: port("Storage") },
  { object: "window", property: "sessionStorage", message: port("Storage") },
  { object: "globalThis", property: "setTimeout", message: port("Clock (setTimeout)") },
  { object: "globalThis", property: "setInterval", message: port("Clock (setTimeout)") },
  { object: "globalThis", property: "fetch", message: port("Http") },
  { object: "globalThis", property: "localStorage", message: port("Storage") },
  { object: "globalThis", property: "sessionStorage", message: port("Storage") },
];

const restrictedImports = {
  paths: ["fs", "node:fs", "fs/promises", "node:fs/promises", "child_process", "node:child_process"].map((name) => ({
    name,
    message: "外 (file・子 process) を直に触っている。outside の口を通す (test では mock を渡す)",
  })),
};

export default [
  {
    files: ["**/*.ts", "**/*.tsx", "**/*.js", "**/*.mjs"],
    rules: {
      "no-restricted-syntax": ["error", ...restrictedSyntax],
      "no-restricted-globals": ["error", ...restrictedGlobals],
      "no-restricted-properties": ["error", ...restrictedProperties],
      "no-restricted-imports": ["error", restrictedImports],
    },
  },
];
