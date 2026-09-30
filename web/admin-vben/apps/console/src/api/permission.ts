import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

export interface APIPermissionTreeNode {
  children?: APIPermissionTreeNode[];
  identifier?: string;
  key: string;
  method?: string;
  path?: string;
  title: string;
}

export function getApiPermissionOptions() {
  return requestClient.get<APIPermissionTreeNode[]>(
    consoleEndpoint('consoleListAPIPermissions'),
  );
}
