import type { RouteRecordRaw } from 'vue-router';

import { describe, expect, it } from 'vitest';

const routeModules = import.meta.glob('./*.ts', { eager: true }) as Record<
  string,
  { default?: RouteRecordRaw[] }
>;

function allRoutes(): RouteRecordRaw[] {
  return Object.entries(routeModules)
    .filter(([path]) => !path.endsWith('.test.ts'))
    .flatMap(([, module]) => module.default ?? []);
}

// Route modules are concatenated, not merged by path. A /system route declared
// in a second module therefore renders as a top-level menu item instead of
// joining 系统管理 — which is exactly what happened when the log and scheduled
// task pages were added in modules of their own.
describe('system route ownership', () => {
  it('declares every /system page under one parent in one module', () => {
    const topLevel = allRoutes();

    const systemParents = topLevel.filter((route) => route.path === '/system');
    expect(systemParents).toHaveLength(1);

    const strays = topLevel
      .filter((route) => route.path.startsWith('/system/'))
      .map((route) => route.path);
    expect(strays).toEqual([]);
  });

  it('keeps the system children reachable with distinct names and paths', () => {
    const parent = allRoutes().find((route) => route.path === '/system');
    const children = parent?.children ?? [];
    expect(children.length).toBeGreaterThan(0);

    for (const child of children) {
      expect(child.path.startsWith('/system/')).toBe(true);
      expect(child.component).toBeDefined();
      expect(child.meta?.title).toBeTruthy();
    }

    const paths = children.map((child) => child.path);
    const names = children.map((child) => String(child.name));
    expect(new Set(paths).size).toBe(paths.length);
    expect(new Set(names).size).toBe(names.length);
  });
});
