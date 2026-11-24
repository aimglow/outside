import { readFileSync } from "node:fs";
import { execSync } from "child_process";

export function bad() {
  const a = Date.now();
  const b = new Date();
  setTimeout(() => {}, 1);
  setInterval(() => {}, 1);
  const c = process.env.HOME;
  fetch("https://example.invalid");
  localStorage.getItem("x");
  sessionStorage.getItem("x");
  window.setTimeout(() => {}, 1);
  return [a, b, c, readFileSync, execSync];
}
