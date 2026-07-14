<script setup lang="ts">
import type { ConsoleSession } from '#/api/core/console';

import { computed, onMounted, ref } from 'vue';

import { Modal, message } from 'ant-design-vue';

import { getSessionList, revokeSession } from '#/api/core/console';
import { useAuthStore } from '#/store';

defineOptions({ name: 'ConsoleSessions' });

const authStore = useAuthStore();
const loading = ref(false);
const sessions = ref<ConsoleSession[]>([]);
const filters = ref({ keyword: '', status: 'active' });
const pagination = ref({ current: 1, pageSize: 20, total: 0 });

const activeCount = computed(
  () => sessions.value.filter((item) => item.status === 'active').length,
);
const currentDevice = computed(() =>
  sessions.value.find((item) => item.current),
);

const columns = [
  { title: '管理员', dataIndex: 'admin', key: 'admin', width: 180 },
  { title: '设备', dataIndex: 'device_name', key: 'device_name', width: 220 },
  { title: 'IP 地址', dataIndex: 'client_ip', key: 'client_ip', width: 150 },
  {
    title: '最后活跃',
    dataIndex: 'last_active_at',
    key: 'last_active_at',
    width: 190,
  },
  { title: '过期时间', dataIndex: 'expires_at', key: 'expires_at', width: 190 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 110 },
  { title: '操作', key: 'action', fixed: 'right', width: 130 },
];

async function loadSessions() {
  loading.value = true;
  try {
    const result = await getSessionList({
      keyword: filters.value.keyword || undefined,
      page: pagination.value.current,
      page_size: pagination.value.pageSize,
      status: filters.value.status || undefined,
    });
    sessions.value = result.list || [];
    pagination.value.total = result.meta?.total || 0;
  } finally {
    loading.value = false;
  }
}

function handleSearch() {
  pagination.value.current = 1;
  void loadSessions();
}

function handleReset() {
  filters.value = { keyword: '', status: 'active' };
  handleSearch();
}

function handleTableChange(next: { current?: number; pageSize?: number }) {
  pagination.value.current = next.current || 1;
  pagination.value.pageSize = next.pageSize || 20;
  void loadSessions();
}

function handleRevoke(record: ConsoleSession) {
  Modal.confirm({
    title: record.current ? '退出当前设备？' : '强制下线该会话？',
    content: record.current
      ? '当前设备会立即退出管理后台，需要重新登录。'
      : `将撤销 ${record.admin?.display_name || record.admin?.account || '该管理员'} 在 ${record.device_name} 上的登录状态。`,
    okText: record.current ? '退出登录' : '强制下线',
    okType: 'danger',
    async onOk() {
      await revokeSession(record.id);
      message.success(record.current ? '当前会话已退出' : '会话已强制下线');
      if (record.current) {
        await authStore.logout(true, false);
        return;
      }
      await loadSessions();
    },
  });
}

function statusMeta(status: ConsoleSession['status']) {
  if (status === 'active') return { color: 'success', text: '在线' };
  if (status === 'revoked') return { color: 'error', text: '已撤销' };
  return { color: 'default', text: '已过期' };
}

onMounted(loadSessions);
</script>

<template>
  <div class="session-page">
    <div class="session-hero">
      <div>
        <div class="session-kicker">SECURITY / ACTIVE SESSIONS</div>
        <h1>在线会话</h1>
        <p>查看管理后台登录设备，并立即撤销异常或不再使用的会话。</p>
      </div>
      <div class="session-metrics">
        <div class="metric">
          <span>本页在线</span>
          <strong>{{ activeCount }}</strong>
        </div>
        <div class="metric metric-current">
          <span>当前设备</span>
          <strong>{{ currentDevice?.device_name || '未识别' }}</strong>
        </div>
      </div>
    </div>

    <a-card :bordered="false" class="session-card">
      <a-form layout="inline" class="mb-5">
        <a-form-item label="关键词">
          <a-input
            v-model:value="filters.keyword"
            allow-clear
            placeholder="管理员、设备或 IP"
            style="width: 240px"
            @press-enter="handleSearch"
          />
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="filters.status" style="width: 140px">
            <a-select-option value="active">在线</a-select-option>
            <a-select-option value="revoked">已撤销</a-select-option>
            <a-select-option value="expired">已过期</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" @click="handleSearch">查询</a-button>
            <a-button @click="handleReset">重置</a-button>
          </a-space>
        </a-form-item>
      </a-form>

      <a-table
        row-key="id"
        :columns="columns"
        :data-source="sessions"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1180 }"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'admin'">
            <div class="admin-cell">
              <strong>{{ record.admin?.display_name || record.admin?.account || '-' }}</strong>
              <span>{{ record.admin?.account || record.admin_id }}</span>
            </div>
          </template>
          <template v-else-if="column.key === 'device_name'">
            <div class="device-cell">
              <span>{{ record.device_name }}</span>
              <a-tag v-if="record.current" color="blue">当前设备</a-tag>
            </div>
          </template>
          <template v-else-if="column.key === 'status'">
            <a-tag :color="statusMeta(record.status).color">
              {{ statusMeta(record.status).text }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'action'">
            <a-button
              v-if="record.status === 'active'"
              danger
              type="link"
              @click="handleRevoke(record)"
            >
              {{ record.current ? '退出' : '强制下线' }}
            </a-button>
            <span v-else class="text-gray-400">不可操作</span>
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<style scoped>
.session-page {
  min-height: 100%;
  padding: 24px;
  background:
    radial-gradient(circle at 92% 0%, rgb(22 119 255 / 10%), transparent 28%),
    transparent;
}

.session-hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 32px;
  margin-bottom: 20px;
  padding: 8px 4px;
}

.session-hero h1 {
  margin: 6px 0 4px;
  font-size: 28px;
  font-weight: 680;
  letter-spacing: -0.04em;
}

.session-hero p,
.admin-cell span,
.session-kicker,
.metric span {
  color: rgb(100 116 139);
}

.session-kicker {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.16em;
}

.session-metrics {
  display: flex;
  gap: 10px;
}

.metric {
  min-width: 112px;
  padding: 12px 16px;
  border: 1px solid rgb(148 163 184 / 24%);
  border-radius: 12px;
  background: rgb(255 255 255 / 68%);
  backdrop-filter: blur(12px);
}

.metric span,
.metric strong {
  display: block;
}

.metric strong {
  margin-top: 4px;
  font-size: 18px;
}

.metric-current {
  min-width: 220px;
}

.session-card {
  overflow: hidden;
  border-radius: 14px;
  box-shadow: 0 18px 50px rgb(15 23 42 / 6%);
}

.admin-cell,
.device-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.admin-cell span {
  font-size: 12px;
}

.device-cell {
  flex-direction: row;
  align-items: center;
  gap: 8px;
}

@media (max-width: 900px) {
  .session-hero {
    align-items: stretch;
    flex-direction: column;
  }

  .session-metrics {
    overflow-x: auto;
  }
}
</style>
