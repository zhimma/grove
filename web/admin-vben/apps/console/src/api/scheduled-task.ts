import type { ConsoleListResult } from '#/api/core/console';

import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

export interface ScheduledTask {
  id: string;
  name: string;
  display_name: string;
  schedule: string;
  enabled: boolean;
  mutex: boolean;
  timeout_seconds: number;
  /** 已提交但 Worker 尚未认领的手动执行请求 */
  run_requested_at: string;
  last_run_at: string;
  last_status: '' | 'failed' | 'skipped' | 'success';
  last_error: string;
  last_duration_ms: number;
  updated_at: string;
}

export interface ScheduledTaskListParams {
  page?: number;
  page_size?: number;
  keyword?: string;
  enabled?: boolean;
}

export interface UpdateScheduledTaskPayload {
  schedule?: string;
  mutex?: boolean;
  timeout_seconds?: number;
}

// 获取计划任务列表
export function getScheduledTaskList(params: ScheduledTaskListParams) {
  return requestClient.get<ConsoleListResult<ScheduledTask>>(
    consoleEndpoint('consoleListScheduledTasks'),
    { params },
  );
}

// 更新调度参数（表达式 / 互斥 / 超时）
export function updateScheduledTask(
  id: string,
  data: UpdateScheduledTaskPayload,
) {
  return requestClient.put<ScheduledTask>(
    consoleEndpoint('consoleUpdateScheduledTask', { id }),
    data,
  );
}

// 启用或停用任务
export function setScheduledTaskStatus(id: string, enabled: boolean) {
  return requestClient.put<ScheduledTask>(
    consoleEndpoint('consoleSetScheduledTaskStatus', { id }),
    { enabled },
  );
}

// 请求执行一次。返回即表示已登记，实际执行由 Worker 在下一轮对账时认领
export function runScheduledTask(id: string) {
  return requestClient.post<ScheduledTask>(
    consoleEndpoint('consoleRunScheduledTask', { id }),
  );
}
