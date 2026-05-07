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
  admin_id?: string;
  keyword?: string;
  method?: string;
  status?: number;
  start_time?: string;
  end_time?: string;
}

// 获取操作日志列表
export function getOperationLogList(params: LogListParams) {
  return requestClient.get<{ list: OperationLog[]; total: number }>(
    '/console/v1/logs/operations',
    { params },
  );
}

// 获取操作日志详情
export function getOperationLogDetail(id: string) {
  return requestClient.get<OperationLog>(`/console/v1/logs/operations/${id}`);
}

// 删除操作日志
export function deleteOperationLog(id: string) {
  return requestClient.delete(`/console/v1/logs/operations/${id}`);
}

// 清空操作日志
export function clearOperationLog(days?: number) {
  return requestClient.post('/console/v1/logs/operations/clear', { days });
}

// 获取登录日志列表
export function getLoginLogList(params: LogListParams) {
  return requestClient.get<{ list: LoginLog[]; total: number }>(
    '/console/v1/logs/logins',
    { params },
  );
}

// 获取登录日志详情
export function getLoginLogDetail(id: string) {
  return requestClient.get<LoginLog>(`/console/v1/logs/logins/${id}`);
}

// 删除登录日志
export function deleteLoginLog(id: string) {
  return requestClient.delete(`/console/v1/logs/logins/${id}`);
}

// 清空登录日志
export function clearLoginLog(days?: number) {
  return requestClient.post('/console/v1/logs/logins/clear', { days });
}
