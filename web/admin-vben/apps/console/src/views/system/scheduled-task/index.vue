<script setup lang="ts">
import type { ScheduledTask } from '#/api/scheduled-task';

import { onMounted, reactive, ref } from 'vue';

import { message } from 'ant-design-vue';

import {
  getScheduledTaskList,
  runScheduledTask,
  setScheduledTaskStatus,
  updateScheduledTask,
} from '#/api/scheduled-task';

const loading = ref(false);
const taskList = ref<ScheduledTask[]>([]);
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
});

const filters = reactive({
  keyword: '',
  enabled: undefined as boolean | undefined,
});

const editVisible = ref(false);
const editSaving = ref(false);
const editingTask = ref<null | ScheduledTask>(null);
const editForm = reactive({
  schedule: '',
  mutex: true,
  timeout_seconds: 0,
});

const statusOptions = [
  { label: '已启用', value: true },
  { label: '已停用', value: false },
];

const columns = [
  { title: '任务', dataIndex: 'display_name', key: 'display_name', width: 180 },
  { title: '标识', dataIndex: 'name', key: 'name', width: 220 },
  { title: '调度表达式', dataIndex: 'schedule', key: 'schedule', width: 160 },
  { title: '状态', dataIndex: 'enabled', key: 'enabled', width: 100 },
  { title: '上次执行', key: 'last_run', width: 260 },
  { title: '操作', key: 'actions', width: 200, fixed: 'right' },
];

const lastStatusMeta: Record<string, { color: string; text: string }> = {
  failed: { color: 'error', text: '失败' },
  skipped: { color: 'warning', text: '跳过' },
  success: { color: 'success', text: '成功' },
};

async function loadTaskList() {
  loading.value = true;
  try {
    const res = await getScheduledTaskList({
      page: pagination.current,
      page_size: pagination.pageSize,
      keyword: filters.keyword,
      enabled: filters.enabled,
    });
    taskList.value = res.list || [];
    pagination.total = res.meta?.total || 0;
  } finally {
    loading.value = false;
  }
}

function handleTableChange(p: any) {
  pagination.current = p.current;
  pagination.pageSize = p.pageSize;
  loadTaskList();
}

function handleSearch() {
  pagination.current = 1;
  loadTaskList();
}

function handleReset() {
  filters.keyword = '';
  filters.enabled = undefined;
  handleSearch();
}

function handleEdit(record: ScheduledTask) {
  editingTask.value = record;
  editForm.schedule = record.schedule;
  editForm.mutex = record.mutex;
  editForm.timeout_seconds = record.timeout_seconds;
  editVisible.value = true;
}

async function handleEditSubmit() {
  if (!editingTask.value) {
    return;
  }
  editSaving.value = true;
  try {
    await updateScheduledTask(editingTask.value.id, {
      schedule: editForm.schedule,
      mutex: editForm.mutex,
      timeout_seconds: editForm.timeout_seconds,
    });
    message.success('调度已更新，Worker 将在下一轮对账后生效');
    editVisible.value = false;
    loadTaskList();
  } finally {
    editSaving.value = false;
  }
}

async function handleToggle(record: ScheduledTask, enabled: boolean) {
  await setScheduledTaskStatus(record.id, enabled);
  message.success(enabled ? '任务已启用' : '任务已停用');
  loadTaskList();
}

async function handleRun(record: ScheduledTask) {
  await runScheduledTask(record.id);
  message.success('已登记执行请求，Worker 将在下一轮对账时执行');
  loadTaskList();
}

onMounted(() => {
  loadTaskList();
});
</script>

<template>
  <div class="scheduled-task">
    <a-card>
      <template #title>
        <div class="flex items-center justify-between">
          <span>计划任务</span>
        </div>
      </template>

      <a-alert
        class="mb-4"
        type="info"
        show-icon
        message="任务内容在代码中定义，此处只能调整执行时机。修改和手动执行由 Worker 定期对账生效，通常有几十秒延迟。"
      />

      <a-form layout="inline" class="mb-4">
        <a-form-item label="关键词">
          <a-input
            v-model:value="filters.keyword"
            placeholder="搜索任务名称或标识"
            allow-clear
            @press-enter="handleSearch"
          />
        </a-form-item>
        <a-form-item label="状态">
          <a-select
            v-model:value="filters.enabled"
            placeholder="选择状态"
            :options="statusOptions"
            allow-clear
            style="width: 120px"
          />
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" @click="handleSearch">搜索</a-button>
            <a-button @click="handleReset">重置</a-button>
          </a-space>
        </a-form-item>
      </a-form>

      <a-table
        :columns="columns"
        :data-source="taskList"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1120 }"
        row-key="id"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'schedule'">
            <a-typography-text code>{{ record.schedule }}</a-typography-text>
          </template>

          <template v-if="column.key === 'enabled'">
            <a-tag :color="record.enabled ? 'success' : 'default'">
              {{ record.enabled ? '已启用' : '已停用' }}
            </a-tag>
          </template>

          <template v-if="column.key === 'last_run'">
            <div v-if="record.last_run_at">
              <a-space>
                <a-tag
                  :color="
                    lastStatusMeta[record.last_status]?.color ?? 'default'
                  "
                >
                  {{ lastStatusMeta[record.last_status]?.text ?? '未知' }}
                </a-tag>
                <span>{{ record.last_run_at }}</span>
                <span class="text-gray-400">
                  {{ record.last_duration_ms }}ms
                </span>
              </a-space>
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

          <template v-if="column.key === 'actions'">
            <a-space>
              <a-button
                type="link"
                size="small"
                :disabled="!record.enabled || !!record.run_requested_at"
                @click="handleRun(record)"
              >
                {{ record.run_requested_at ? '待执行' : '执行一次' }}
              </a-button>
              <a-button type="link" size="small" @click="handleEdit(record)">
                编辑
              </a-button>
              <a-button
                type="link"
                size="small"
                :danger="record.enabled"
                @click="handleToggle(record, !record.enabled)"
              >
                {{ record.enabled ? '停用' : '启用' }}
              </a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-modal
      v-model:open="editVisible"
      :title="`编辑调度 - ${editingTask?.display_name ?? ''}`"
      :confirm-loading="editSaving"
      @ok="handleEditSubmit"
    >
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="调度表达式">
          <a-input
            v-model:value="editForm.schedule"
            placeholder="秒 分 时 日 月 周，例如 0 17 3 * * *"
          />
          <div class="mt-1 text-xs text-gray-400">
            6 段格式（含秒），也支持 @every 1h 这类写法
          </div>
        </a-form-item>
        <a-form-item label="互斥执行">
          <a-switch v-model:checked="editForm.mutex" />
          <div class="mt-1 text-xs text-gray-400">
            开启后上一次未结束时跳过本次；多实例部署下为集群级互斥
          </div>
        </a-form-item>
        <a-form-item label="超时（秒）">
          <a-input-number v-model:value="editForm.timeout_seconds" :min="0" />
          <div class="mt-1 text-xs text-gray-400">0 表示不限制</div>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<style scoped>
.scheduled-task {
  padding: 24px;
}
</style>
