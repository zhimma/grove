<script setup lang="ts">
import type { LoginLog } from '#/api/log';

import { onMounted, ref } from 'vue';

import { getLoginLogList } from '#/api/log';

import LogDetail from './components/LogDetail.vue';

// 状态
const loading = ref(false);
const logList = ref<LoginLog[]>([]);
const pagination = ref({
  current: 1,
  pageSize: 10,
  total: 0,
});

// 筛选条件
const filters = ref({
  admin_id: '',
  keyword: '',
  success: undefined as boolean | undefined,
  dateRange: [] as string[],
});

// 详情弹窗
const detailVisible = ref(false);
const currentLog = ref<LoginLog | null>(null);

// 表格列
const columns = [
  {
    title: '管理员',
    dataIndex: 'admin_name',
    key: 'admin_name',
    width: 120,
  },
  {
    title: '登录账号',
    dataIndex: 'account',
    key: 'account',
    width: 150,
  },
  {
    title: '登录状态',
    dataIndex: 'success',
    key: 'success',
    width: 100,
  },
  {
    title: '失败原因',
    dataIndex: 'failure_reason',
    key: 'failure_reason',
    ellipsis: true,
  },
  {
    title: 'IP地址',
    dataIndex: 'client_ip',
    key: 'client_ip',
    width: 130,
  },
  {
    title: '登录时间',
    dataIndex: 'created_at',
    key: 'created_at',
    width: 180,
  },
  {
    title: '操作',
    key: 'actions',
    width: 150,
    fixed: 'right',
  },
];

// 状态选项
const successOptions = [
  { label: '成功', value: true },
  { label: '失败', value: false },
];

// 加载日志列表
async function loadLogList() {
  loading.value = true;
  try {
    const res = await getLoginLogList({
      page: pagination.value.current,
      page_size: pagination.value.pageSize,
      admin_id: filters.value.admin_id,
      keyword: filters.value.keyword,
      success: filters.value.success,
      created_from: filters.value.dateRange[0],
      created_to: filters.value.dateRange[1],
    });
    logList.value = res.list || [];
    pagination.value.total = res.meta?.total || 0;
  } finally {
    loading.value = false;
  }
}

// 表格变化
function handleTableChange(p: any) {
  pagination.value.current = p.current;
  pagination.value.pageSize = p.pageSize;
  loadLogList();
}

// 搜索
function handleSearch() {
  pagination.value.current = 1;
  loadLogList();
}

// 重置筛选
function handleReset() {
  filters.value = {
    admin_id: '',
    keyword: '',
    success: undefined,
    dateRange: [],
  };
  handleSearch();
}

// 查看详情
function handleDetail(record: LoginLog) {
  currentLog.value = record;
  detailVisible.value = true;
}

onMounted(() => {
  loadLogList();
});
</script>

<template>
  <div class="login-log">
    <a-card>
      <template #title>
        <div class="flex items-center justify-between">
          <span>登录日志</span>
        </div>
      </template>

      <!-- 筛选区域 -->
      <a-form layout="inline" class="mb-4">
        <a-form-item label="关键词">
          <a-input
            v-model:value="filters.keyword"
            placeholder="搜索账号/IP"
            allow-clear
            @press-enter="handleSearch"
          />
        </a-form-item>
        <a-form-item label="状态">
          <a-select
            v-model:value="filters.success"
            placeholder="选择状态"
            :options="successOptions"
            allow-clear
            style="width: 120px"
          />
        </a-form-item>
        <a-form-item label="时间范围">
          <a-range-picker
            v-model:value="filters.dateRange"
            show-time
            format="YYYY-MM-DD HH:mm:ss"
          />
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" @click="handleSearch">搜索</a-button>
            <a-button @click="handleReset">重置</a-button>
          </a-space>
        </a-form-item>
      </a-form>

      <!-- 日志列表 -->
      <a-table
        :columns="columns"
        :data-source="logList"
        :loading="loading"
        :pagination="pagination"
        @change="handleTableChange"
        row-key="id"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'success'">
            <a-tag :color="record.success ? 'success' : 'error'">
              {{ record.success ? '成功' : '失败' }}
            </a-tag>
          </template>

          <template v-if="column.key === 'actions'">
            <a-space>
              <a-button type="link" size="small" @click="handleDetail(record)">
                详情
              </a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <!-- 详情弹窗 -->
    <LogDetail v-model:visible="detailVisible" :log="currentLog" type="login" />
  </div>
</template>

<style scoped>
.login-log {
  padding: 24px;
}
</style>
