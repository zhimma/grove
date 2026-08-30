import type { ConsoleListResult } from '#/api/core/console';

import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

export interface OperationLog {
  id: string;
  admin_id: string;
  admin_account: string;
  admin_name: string;
  method: string;
  path: string;
  route: string;
  module: string;
  action: string;
  target_type: string;
  target_id: string;
  request_id: string;
  status_code: number;
  success: boolean;
  error_message: string;
  duration_ms: number;
  client_ip: string;
  user_agent: string;
  request_query: string;
  created_at: string;
}

export interface OperationLogDetail {
  log: OperationLog;
  detail: Record<string, unknown>;
}

export interface LoginLog {
  id: string;
  admin_id: string;
  admin_account: string;
  admin_name: string;
  account: string;
  success: boolean;
  failure_reason: string;
  request_id: string;
  client_ip: string;
  user_agent: string;
  created_at: string;
}

export interface LogListParams {
  page?: number;
  page_size?: number;
  offset?: number;
  limit?: number;
  list_all?: boolean;
  admin_id?: string;
  keyword?: string;
  method?: string;
  module?: string;
  success?: boolean;
  order_by?: string | string[];
  created_from?: string;
  created_to?: string;
}

// 获取操作日志列表
export function getOperationLogList(params: LogListParams) {
  return requestClient.get<ConsoleListResult<OperationLog>>(
    consoleEndpoint('consoleListOperationLogs'),
    { params },
  );
}

// 获取操作日志详情
export function getOperationLogDetail(id: string) {
  return requestClient.get<OperationLogDetail>(
    consoleEndpoint('consoleGetOperationLog', { id }),
  );
}

// 获取登录日志列表
export function getLoginLogList(params: LogListParams) {
  return requestClient.get<ConsoleListResult<LoginLog>>(
    consoleEndpoint('consoleListLoginLogs'),
    { params },
  );
}
