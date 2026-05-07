import type { PageParams } from '#/types/common';

import { requestClient } from '#/api/request';

export interface ConsoleListMeta {
  total: number;
  page: number;
  page_size: number;
  total_pages?: number;
}

export interface ConsoleListResult<T> {
  list: T[];
  meta: ConsoleListMeta;
}

export interface DashboardOverview {
  admin_count: number;
  role_count: number;
  operation_count: number;
  login_count: number;
  message?: string;
}

export interface ConsoleAdmin {
  id: string;
  account: string;
  username?: string;
  real_name?: string;
  display_name?: string;
  phone?: string;
  email?: string;
  status: number;
  is_super: boolean;
  role?: { display_name?: string; id: string; name: string };
  created_at: string;
  last_login_at?: string;
}

export interface SystemConfigRecord {
  id: string;
  config_group: string;
  config_key: string;
  name: string;
  description?: string;
  value_type: string;
  value: string;
  is_editable: boolean;
  is_system?: boolean;
  updated_at: string;
}

export function getDashboardOverview() {
  return requestClient.get<DashboardOverview>('/console/v1/dashboard/summary');
}

export function getAdminList(params: PageParams & Record<string, any>) {
  return requestClient.get<ConsoleListResult<ConsoleAdmin>>(
    '/console/v1/admins',
    { params },
  );
}

export function createAdmin(data: Record<string, any>) {
  return requestClient.post<ConsoleAdmin>('/console/v1/admins', data);
}

export function updateAdmin(id: string, data: Record<string, any>) {
  return requestClient.put<ConsoleAdmin>(`/console/v1/admins/${id}`, data);
}

export function updateAdminStatus(id: string, status: number) {
  return requestClient.put(`/console/v1/admins/${id}/status`, { status });
}

export function resetAdminPassword(id: string, password: string) {
  return requestClient.put(`/console/v1/admins/${id}/reset-password`, {
    password,
  });
}

export function deleteAdmin(id: string) {
  return requestClient.delete(`/console/v1/admins/${id}`);
}

export function getSystemConfigList(params: PageParams & Record<string, any>) {
  return requestClient.get<ConsoleListResult<SystemConfigRecord>>(
    '/console/v1/system-configs',
    { params },
  );
}

export function updateSystemConfig(id: string, data: Record<string, any>) {
  return requestClient.put<SystemConfigRecord>(
    `/console/v1/system-configs/${id}`,
    data,
  );
}

export function createSystemConfig(data: Record<string, any>) {
  return requestClient.post<SystemConfigRecord>(
    '/console/v1/system-configs',
    data,
  );
}

export function deleteSystemConfig(id: string) {
  return requestClient.delete(`/console/v1/system-configs/${id}`);
}
