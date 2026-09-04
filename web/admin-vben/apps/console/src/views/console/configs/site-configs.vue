<script setup lang="ts">
import {
  createSystemConfig,
  deleteSystemConfig,
  getSystemConfigList,
  updateSystemConfig,
} from '#/api/core/console';
import ResourcePage from '#/components/resource-page/index.vue';

const columns = [
  { title: '配置键', dataIndex: 'config_key', key: 'config_key', width: 220 },
  { title: '名称', dataIndex: 'name', key: 'name', width: 180 },
  { title: '值', dataIndex: 'value', key: 'value', width: 300, ellipsis: true },
  { title: '类型', dataIndex: 'value_type', key: 'value_type', width: 100 },
  { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 180 },
];

function normalizePayload(payload: Record<string, any>) {
  return { ...payload, config_group: 'site' };
}
</script>

<template>
  <ResourcePage
    title="站点配置"
    :columns="columns"
    :search-fields="[{ key: 'keyword', label: '关键词' }]"
    :has-list-params="{ config_group: 'site' }"
    component-name="system-config-detail"
    :fetch-api="getSystemConfigList"
    :create-api="createSystemConfig"
    :update-api="updateSystemConfig"
    :delete-api="deleteSystemConfig"
    :has-custom-submit-fun="normalizePayload"
  />
</template>
