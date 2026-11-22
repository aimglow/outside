import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    include: ['test/**/*.test.ts'],
    // 時刻の表示が機体の timezone に依らないよう UTC に固定する。
    env: { TZ: 'UTC' },
  },
});
