<script setup lang="ts">
import type { ScheduledTask } from '#/api/scheduled-task';

import { ref } from 'vue';

import { Alert, Button, message, Tag, TypographyText } from 'ant-design-vue';

import {
  getScheduledTaskList,
  runScheduledTask,
  setScheduledTaskStatus,
  updateScheduledTask,
} from '#/api/scheduled-task';
import ResourcePage from '#/components/resource-page/index.vue';

const pageRef = ref<InstanceType<typeof ResourcePage>>();

const columns = [
  { title: '任务', dataIndex: 'display_name', key: 'display_name', width: 180 },
  { title: '标识', dataIndex: 'name', key: 'name', width: 220 },
  { title: '调度表达式', dataIndex: 'schedule', key: 'schedule', width: 160 },
  { title: '状态', dataIndex: 'enabled', key: 'enabled', width: 100 },
  { title: '上次执行', dataIndex: 'last_run_at', key: 'last_run', width: 260 },
];

const lastStatusMeta: Record<string, { color: string; text: string }> = {
  failed: { color: 'error', text: '失败' },
  skipped: { color: 'warning', text: '跳过' },
  success: { color: 'success', text: '成功' },
};

// The task body lives in code, so only the timing is editable.
function toUpdatePayload(data: Record<string, any>) {
  return {
    mutex: data.mutex,
    schedule: data.schedule,
    timeout_seconds: data.timeout_seconds,
  };
}

async function handleToggle(task: ScheduledTask) {
  await setScheduledTaskStatus(task.id, !task.enabled);
  message.success(task.enabled ? '任务已停用' : '任务已启用');
  await pageRef.value?.reload();
}

async function handleRun(task: ScheduledTask) {
  await runScheduledTask(task.id);
  message.success('已登记执行请求，Worker 将在下一轮对账时执行');
  await pageRef.value?.reload();
}
</script>

<template>
  <div>
    <Alert
      class="mb-4"
      type="info"
      show-icon
      message="任务内容在代码中定义，此处只能调整执行时机。修改、启停和手动执行由 Worker 定期对账生效，通常有几十秒延迟。"
    />
    <ResourcePage
      ref="pageRef"
      title="计划任务"
      :action-width="200"
      :columns="columns"
      :search-fields="[
        { key: 'keyword', label: '关键词' },
        {
          key: 'enabled',
          label: '状态',
          type: 'select',
          options: [
            { label: '已启用', value: 'true' },
            { label: '已停用', value: 'false' },
          ],
        },
      ]"
      :form-fields="[
        {
          key: 'schedule',
          label: '调度表达式',
          required: true,
          placeholder: '秒 分 时 日 月 周，例如 0 17 3 * * *',
          help: '6 段格式（含秒），也支持 @every 1h 这类写法',
        },
        {
          key: 'mutex',
          label: '互斥执行',
          type: 'switch',
          help: '开启后上一次未结束时跳过本次；多实例部署下为集群级互斥',
        },
        {
          key: 'timeout_seconds',
          label: '超时（秒）',
          type: 'number',
          help: '0 表示不限制',
        },
      ]"
      :fetch-api="getScheduledTaskList"
      :update-api="updateScheduledTask"
      :transform-payload="toUpdatePayload"
    >
      <template #actions="{ record }">
        <Button
          type="link"
          size="small"
          :disabled="!record.enabled || !!record.run_requested_at"
          @click="handleRun(record as ScheduledTask)"
        >
          {{ record.run_requested_at ? '待执行' : '执行一次' }}
        </Button>
        <Button
          type="link"
          size="small"
          :danger="record.enabled"
          @click="handleToggle(record as ScheduledTask)"
        >
          {{ record.enabled ? '停用' : '启用' }}
        </Button>
      </template>
      <template #cell="{ column, record }">
        <TypographyText v-if="column.key === 'schedule'" code>
          {{ record.schedule }}
        </TypographyText>
        <Tag
          v-else-if="column.key === 'enabled'"
          :color="record.enabled ? 'success' : 'default'"
        >
          {{ record.enabled ? '已启用' : '已停用' }}
        </Tag>
        <template v-else-if="column.key === 'last_run'">
          <div v-if="record.last_run_at">
            <Tag
              :color="lastStatusMeta[record.last_status]?.color ?? 'default'"
            >
              {{ lastStatusMeta[record.last_status]?.text ?? '未知' }}
            </Tag>
            <span>{{ record.last_run_at }}</span>
            <span class="ml-2 text-gray-400">
              {{ record.last_duration_ms }}ms
            </span>
            <div
              v-if="record.last_error"
              class="mt-1 truncate text-xs text-red-500"
              :title="record.last_error"
            >
              {{ record.last_error }}
            </div>
          </div>
          <span v-else class="text-gray-400">尚未执行</span>
        </template>
      </template>
    </ResourcePage>
  </div>
</template>
