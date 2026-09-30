import type { PageData, PageParams } from '#/types/pagination';

import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

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

export function getSessionList(params: PageParams & Record<string, any>) {
  return requestClient.get<PageData<ConsoleSession>>(
    consoleEndpoint('consoleListSessions'),
    { params },
  );
}

export function revokeSession(id: string) {
  return requestClient.delete(consoleEndpoint('consoleRevokeSession', { id }));
}
