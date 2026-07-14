import type { Component } from 'vue';
import type { RouteRecordRaw } from 'vue-router';

import { describe, expect, it } from 'vitest';

import {
  buildConsoleMenuPermissionTree,
  filterConsoleRoutesByMenuKeys,
} from './menu-access';

const routes: RouteRecordRaw[] = [
  {
    name: 'ConsoleFuture',
    path: '/future',
    meta: { title: '未来模块' },
    children: [
      {
        name: 'ConsoleFutureReport',
        path: '/future/report',
        component: {} as Component,
        meta: { title: '未来报表' },
      },
    ],
  },
];

describe('console menu access', () => {
  it('builds permission options directly from local routes', () => {
    expect(buildConsoleMenuPermissionTree(routes)).toEqual([
      {
        key: 'ConsoleFuture',
        title: '未来模块',
        children: [{ key: 'ConsoleFutureReport', title: '未来报表' }],
      },
    ]);
  });

  it('keeps the parent route when only a child key is granted', () => {
    const filtered = filterConsoleRoutesByMenuKeys(routes, [
      'ConsoleFutureReport',
      'legacy.invalid',
    ]);
    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe('ConsoleFuture');
    expect(filtered[0]?.children?.map((item) => item.name)).toEqual([
      'ConsoleFutureReport',
    ]);
  });
});
