import type { PageData, PageParams } from '#/types/common';

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
  return requestClient.get<PageData<Role>>('/console/v1/roles', {
    params,
  });
}

/**
 * 创建角色
 */
export function createRole(data: CreateRoleParams) {
  return requestClient.post<Role>('/console/v1/roles', data);
}

/**
 * 更新角色
 */
export function updateRole(id: string, data: UpdateRoleParams) {
  return requestClient.put<Role>(`/console/v1/roles/${id}`, data);
}

/**
 * 删除角色
 */
export function deleteRole(id: string) {
  return requestClient.delete<null>(`/console/v1/roles/${id}`);
}

/**
 * 获取角色权限
 */
export function getRolePermissions(id: string) {
  return requestClient.get<string[]>(`/console/v1/roles/${id}/permissions`);
}

/**
 * 分配角色权限
 */
export function assignRolePermissions(
  id: string,
  data: AssignPermissionsParams,
) {
  return requestClient.post<null>(`/console/v1/roles/${id}/permissions`, data);
}

export function getRoleMenus(id: string) {
  return requestClient.get<string[]>(`/console/v1/roles/${id}/menus`);
}

export function assignRoleMenus(id: string, data: AssignMenusParams) {
  return requestClient.post<null>(`/console/v1/roles/${id}/menus`, data);
}
