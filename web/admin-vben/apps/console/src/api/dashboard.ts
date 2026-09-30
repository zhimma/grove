import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

export interface DashboardOverview {
  admin_count: number;
  user_count: number;
  role_count: number;
  operation_count: number;
  login_count: number;
  message?: string;
}

export function getDashboardOverview() {
  return requestClient.get<DashboardOverview>(
    consoleEndpoint('consoleGetDashboardSummary'),
  );
}
