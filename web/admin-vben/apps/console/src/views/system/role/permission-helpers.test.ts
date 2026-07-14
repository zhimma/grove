import type { APIPermissionTreeNode } from '#/api/core/permission';

import { describe, expect, it } from 'vitest';

import { collectLeafPermissionKeys } from './permission-helpers';

const permissionTree: APIPermissionTreeNode[] = [
  {
    key: 'roles',
    title: '角色管理',
    children: [
      { key: 'GET /roles', title: '查看角色' },
      { key: 'POST /roles', title: '创建角色' },
    ],
  },
  { key: 'GET /admins', title: '查看管理员' },
];

describe('collectLeafPermissionKeys', () => {
  it('keeps only selected leaf permissions in catalog order', () => {
    expect(
      collectLeafPermissionKeys(permissionTree, [
        'POST /roles',
        'roles',
        'unknown',
        'GET /roles',
      ]),
    ).toEqual(['GET /roles', 'POST /roles']);
  });

  it('returns an empty list for empty selections', () => {
    expect(collectLeafPermissionKeys(permissionTree, [])).toEqual([]);
  });
});
