import type { PageData, PageParams } from '#/types/pagination';

import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

export interface ConsoleUser {
  id: string;
  name: string;
  email: string;
  phone?: string;
  avatar?: string;
  status: number;
  status_text: string;
  remark?: string;
  last_login_at?: string;
  created_at: string;
  updated_at: string;
}

export function getUserList(params: PageParams & Record<string, any>) {
  return requestClient.get<PageData<ConsoleUser>>(
    consoleEndpoint('consoleListUsers'),
    { params },
  );
}

export function getUser(id: string) {
  return requestClient.get<ConsoleUser>(
    consoleEndpoint('consoleGetUser', { id }),
  );
}

export function createUser(data: Record<string, any>) {
  return requestClient.post<ConsoleUser>(
    consoleEndpoint('consoleCreateUser'),
    data,
  );
}

export function updateUser(id: string, data: Record<string, any>) {
  return requestClient.put<ConsoleUser>(
    consoleEndpoint('consoleUpdateUser', { id }),
    data,
  );
}

export function updateUserStatus(id: string, status: number) {
  return requestClient.put<ConsoleUser>(
    consoleEndpoint('consoleUpdateUserStatus', { id }),
    { status },
  );
}

export function deleteUser(id: string) {
  return requestClient.delete(consoleEndpoint('consoleDeleteUser', { id }));
}
