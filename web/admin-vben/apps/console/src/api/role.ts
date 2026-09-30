import type { PageData, PageParams } from '#/types/pagination';

import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

export interface Role {
  id: string;
  name: string;
  code: string;
  display_name: string;
  description: string;
  status: number;
  is_super: boolean;
  sort: number;
  created_at: string;
  updated_at: string;
}

export interface RoleListParams extends PageParams {
  list_all?: boolean;
  status?: number;
  keyword?: string;
}

export interface CreateRoleParams {
  name: string;
  code: string;
  display_name?: string;
  description?: string;
  sort?: number;
  status?: number;
}

export interface UpdateRoleParams {
  name?: string;
  code?: string;
  display_name?: string;
  description?: string;
  status?: number;
  sort?: number;
}

export interface AssignPermissionsParams {
  api_permissions: string[];
}

export interface AssignMenusParams {
  menu_keys: string[];
}

/**
 * 获取角色列表
 */
export function getRoleList(params: RoleListParams) {
  return requestClient.get<PageData<Role>>(
    consoleEndpoint('consoleListRoles'),
    {
      params,
    },
  );
}

/**
 * 创建角色
 */
export function createRole(data: CreateRoleParams) {
  return requestClient.post<Role>(consoleEndpoint('consoleCreateRole'), data);
}

/**
 * 更新角色
 */
export function updateRole(id: string, data: UpdateRoleParams) {
  return requestClient.put<Role>(
    consoleEndpoint('consoleUpdateRole', { id }),
    data,
  );
}

/**
 * 删除角色
 */
export function deleteRole(id: string) {
  return requestClient.delete<null>(
    consoleEndpoint('consoleDeleteRole', { id }),
  );
}

/**
 * 获取角色权限
 */
export function getRolePermissions(id: string) {
  return requestClient.get<string[]>(
    consoleEndpoint('consoleGetRolePermissions', { id }),
  );
}

/**
 * 分配角色权限
 */
export function assignRolePermissions(
  id: string,
  data: AssignPermissionsParams,
) {
  return requestClient.post<null>(
    consoleEndpoint('consoleAssignRolePermissions', { id }),
    data,
  );
}

export function getRoleMenus(id: string) {
  return requestClient.get<string[]>(
    consoleEndpoint('consoleGetRoleMenus', { id }),
  );
}

export function assignRoleMenus(id: string, data: AssignMenusParams) {
  return requestClient.post<null>(
    consoleEndpoint('consoleAssignRoleMenus', { id }),
    data,
  );
}
