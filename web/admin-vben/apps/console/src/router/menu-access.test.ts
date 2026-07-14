import type { Component } from 'vue';
import type { RouteRecordRaw } from 'vue-router';

import { describe, expect, it } from 'vitest';

import {
  buildConsoleMenuPermissionTree,
  filterConsoleRoutesByMenuKeys,
  findFirstConsoleAccessiblePath,
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

  it('supports wildcard and removes unknown or empty permissions', () => {
    expect(filterConsoleRoutesByMenuKeys(routes, ['*'])).toBe(routes);
    expect(filterConsoleRoutesByMenuKeys(routes, [])).toEqual([]);
    expect(filterConsoleRoutesByMenuKeys(routes, ['legacy.invalid'])).toEqual(
      [],
    );
  });

  it('flattens hidden catalog nodes while preserving visible children', () => {
    const tree = buildConsoleMenuPermissionTree([
      {
        name: 'HiddenParent',
        path: '/hidden',
        meta: { hideInMenu: true, title: '隐藏父级' },
        children: [
          {
            name: 'VisibleChild',
            path: '/hidden/visible',
            component: {} as Component,
            meta: { title: '可见子级' },
          },
        ],
      },
    ]);

    expect(tree).toEqual([{ key: 'VisibleChild', title: '可见子级' }]);
  });

  it('selects the first child path before redirect and route fallback', () => {
    expect(
      findFirstConsoleAccessiblePath([
        {
          path: '/parent',
          redirect: '/parent/default',
          children: [{ path: '/parent/first', component: {} as Component }],
        },
      ]),
    ).toBe('/parent/first');
    expect(
      findFirstConsoleAccessiblePath([
        { path: '/fallback', component: {} as Component },
      ]),
    ).toBe('/fallback');
    expect(findFirstConsoleAccessiblePath([])).toBeNull();
  });
});
