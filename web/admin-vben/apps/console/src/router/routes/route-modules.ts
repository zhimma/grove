import type { RouteRecordRaw } from 'vue-router';

// Tests can live beside routes, but must never execute in the browser bundle.
export const routeModules = import.meta.glob<{ default: RouteRecordRaw[] }>(
  [
    './modules/**/*.ts',
    '!./modules/**/*.{test,spec}.ts',
    '!./modules/**/__tests__/**',
  ],
  { eager: true },
);
