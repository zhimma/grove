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
    '/console/v1/permissions/apis',
  );
}
