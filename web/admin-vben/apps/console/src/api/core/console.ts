import type { PageParams } from '#/types/common';

import { consoleEndpoint } from '#/api/console-contract';
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
  default_value?: string;
  is_editable: boolean;
  is_system?: boolean;
  is_secret: boolean;
  sort_order?: number;
  updated_at: string;
}

export interface ConsoleSession {
  id: string;
  admin_id: string;
  admin?: { account: string; display_name: string; id: string };
  device_name: string;
  client_ip: string;
  user_agent: string;
  last_active_at: string;
  expires_at: string;
  revoked_at?: string;
  revoke_reason?: string;
  status: 'active' | 'expired' | 'revoked';
  current: boolean;
}

export function getDashboardOverview() {
  return requestClient.get<DashboardOverview>(
    consoleEndpoint('consoleGetDashboardSummary'),
  );
}

export function getAdminList(params: PageParams & Record<string, any>) {
  return requestClient.get<ConsoleListResult<ConsoleAdmin>>(
    consoleEndpoint('consoleListAdmins'),
    { params },
  );
}

export function createAdmin(data: Record<string, any>) {
  return requestClient.post<ConsoleAdmin>(
    consoleEndpoint('consoleCreateAdmin'),
    data,
  );
}

export function updateAdmin(id: string, data: Record<string, any>) {
  return requestClient.put<ConsoleAdmin>(
    consoleEndpoint('consoleUpdateAdmin', { id }),
    data,
  );
}

export function updateAdminStatus(id: string, status: number) {
  return requestClient.put(
    consoleEndpoint('consoleUpdateAdminStatus', { id }),
    { status },
  );
}

export function resetAdminPassword(id: string, password: string) {
  return requestClient.put(
    consoleEndpoint('consoleResetAdminPassword', { id }),
    { password },
  );
}

export function deleteAdmin(id: string) {
  return requestClient.delete(consoleEndpoint('consoleDeleteAdmin', { id }));
}

export function getSessionList(params: PageParams & Record<string, any>) {
  return requestClient.get<ConsoleListResult<ConsoleSession>>(
    consoleEndpoint('consoleListSessions'),
    { params },
  );
}

export function revokeSession(id: string) {
  return requestClient.delete(consoleEndpoint('consoleRevokeSession', { id }));
}

export function getSystemConfigList(params: PageParams & Record<string, any>) {
  return requestClient.get<ConsoleListResult<SystemConfigRecord>>(
    consoleEndpoint('consoleListSystemConfigs'),
    { params },
  );
}

export function updateSystemConfig(id: string, data: Record<string, any>) {
  return requestClient.put<SystemConfigRecord>(
    consoleEndpoint('consoleUpdateSystemConfig', { id }),
    data,
  );
}

export function createSystemConfig(data: Record<string, any>) {
  return requestClient.post<SystemConfigRecord>(
    consoleEndpoint('consoleCreateSystemConfig'),
    data,
  );
}

export function deleteSystemConfig(id: string) {
  return requestClient.delete(
    consoleEndpoint('consoleDeleteSystemConfig', { id }),
  );
}
