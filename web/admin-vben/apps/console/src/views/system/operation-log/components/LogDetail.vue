<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import type { OperationLog, LoginLog } from '#/api/log';

interface Props {
  visible: boolean;
  log: OperationLog | LoginLog | null;
  type: 'operation' | 'login';
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:visible': [value: boolean];
}>();

const localVisible = ref(props.visible);

watch(() => props.visible, (val) => {
  localVisible.value = val;
});

watch(() => localVisible.value, (val) => {
  emit('update:visible', val);
});

function handleCancel() {
  localVisible.value = false;
}

// 判断是否为操作日志
const isOperationLog = computed(() => props.type === 'operation' && props.log && 'method' in props.log);

// 判断是否为登录日志
const isLoginLog = computed(() => props.type === 'login' && props.log && 'account' in props.log);
</script>

<template>
  <a-modal
    v-model:visible="localVisible"
    title="日志详情"
    width="700px"
    :footer="null"
    @cancel="handleCancel"
  >
    <a-descriptions v-if="log" :column="1" bordered>
      <!-- 通用字段 -->
      <a-descriptions-item label="日志ID">
        {{ log.id }}
      </a-descriptions-item>
      <a-descriptions-item label="管理员ID">
        {{ log.admin_id }}
      </a-descriptions-item>
      <a-descriptions-item label="管理员账号">
        {{ log.admin_account || log.admin_name }}
      </a-descriptions-item>
      
      <!-- 操作日志特有字段 -->
      <template v-if="isOperationLog">
        <a-descriptions-item label="操作动作">
          {{ (log as OperationLog).action }}
        </a-descriptions-item>
        <a-descriptions-item label="请求方法">
          <a-tag>{{ (log as OperationLog).method }}</a-tag>
        </a-descriptions-item>
        <a-descriptions-item label="请求路径">
          {{ (log as OperationLog).path }}
        </a-descriptions-item>
        <a-descriptions-item label="路由标识">
          {{ (log as OperationLog).route }}
        </a-descriptions-item>
        <a-descriptions-item label="模块">
          {{ (log as OperationLog).module }}
        </a-descriptions-item>
        <a-descriptions-item label="目标类型">
          {{ (log as OperationLog).target_type || '-' }}
        </a-descriptions-item>
        <a-descriptions-item label="目标ID">
          {{ (log as OperationLog).target_id || '-' }}
        </a-descriptions-item>
        <a-descriptions-item label="请求ID">
          {{ (log as OperationLog).request_id }}
        </a-descriptions-item>
        <a-descriptions-item label="状态码">
          <a-tag :color="(log as OperationLog).status_code === 200 ? 'success' : 'error'">
            {{ (log as OperationLog).status_code }}
          </a-tag>
        </a-descriptions-item>
        <a-descriptions-item label="耗时(ms)">
          {{ (log as OperationLog).duration_ms }}ms
        </a-descriptions-item>
        <a-descriptions-item label="请求参数">
          <pre style="max-height: 200px; overflow: auto;">{{ (log as OperationLog).request_query || '-' }}</pre>
        </a-descriptions-item>
        <a-descriptions-item label="错误信息" v-if="!(log as OperationLog).success">
          <span style="color: red;">{{ (log as OperationLog).error_message || '-' }}</span>
        </a-descriptions-item>
      </template>
      
      <!-- 登录日志特有字段 -->
      <template v-if="isLoginLog">
        <a-descriptions-item label="登录账号">
          {{ (log as LoginLog).account }}
        </a-descriptions-item>
        <a-descriptions-item label="登录状态">
          <a-tag :color="(log as LoginLog).success ? 'success' : 'error'">
            {{ (log as LoginLog).success ? '成功' : '失败' }}
          </a-tag>
        </a-descriptions-item>
        <a-descriptions-item label="失败原因" v-if="!(log as LoginLog).success">
          <span style="color: red;">{{ (log as LoginLog).failure_reason || '-' }}</span>
        </a-descriptions-item>
        <a-descriptions-item label="请求ID">
          {{ (log as LoginLog).request_id }}
        </a-descriptions-item>
      </template>
      
      <!-- 通用字段 -->
      <a-descriptions-item label="IP地址">
        {{ log.client_ip }}
      </a-descriptions-item>
      <a-descriptions-item label="User Agent">
        <div style="max-height: 100px; overflow: auto; word-break: break-all;">
          {{ log.user_agent }}
        </div>
      </a-descriptions-item>
      <a-descriptions-item label="创建时间">
        {{ log.created_at }}
      </a-descriptions-item>
    </a-descriptions>
  </a-modal>
</template>
