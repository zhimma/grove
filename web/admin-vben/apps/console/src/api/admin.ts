import type { PageData, PageParams } from '#/types/pagination';

import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

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

export function getAdminList(params: PageParams & Record<string, any>) {
  return requestClient.get<PageData<ConsoleAdmin>>(
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
