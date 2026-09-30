import type { PageData, PageParams } from '#/types/pagination';

import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

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

export function getSystemConfigList(params: PageParams & Record<string, any>) {
  return requestClient.get<PageData<SystemConfigRecord>>(
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
