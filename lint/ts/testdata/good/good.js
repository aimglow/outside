// 外は 口として受け取る。決めた時刻の Date は外ではない。
export function good(clock, http, storage, config) {
  const now = clock.now();
  const fixed = new Date(0);
  clock.setTimeout(() => {}, 1);
  http.request({ url: config.get("API") });
  storage.get("x");
  return [now, fixed];
}
