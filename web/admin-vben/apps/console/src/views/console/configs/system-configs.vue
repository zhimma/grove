<script setup lang="ts">
import {
  createSystemConfig,
  deleteSystemConfig,
  getSystemConfigList,
  updateSystemConfig,
} from '#/api/core/console';

import ResourcePage from '../shared/resource-page.vue';

const columns = [
  { title: '分组', dataIndex: 'config_group', key: 'config_group', width: 120 },
  { title: '配置键', dataIndex: 'config_key', key: 'config_key', width: 180 },
  { title: '名称', dataIndex: 'name', key: 'name', width: 150 },
  {
    title: '值',
    dataIndex: 'value',
    key: 'value',
    width: 220,
    ellipsis: true,
    customRender: ({ text, record }: { record: any; text: string }) => {
      if (record.value_type === 'bool') {
        return text === 'true' ? '是' : '否';
      }
      if (record.value_type === 'json') {
        return text && text.length > 50
          ? `${text.slice(0, 50)}...`
          : text || '{}';
      }
      if (record.value_type === 'array') {
        return text && text.length > 50
          ? `${text.slice(0, 50)}...`
          : text || '[]';
      }
      return text;
    },
  },
  { title: '类型', dataIndex: 'value_type', key: 'value_type', width: 100 },
  {
    title: '可编辑',
    dataIndex: 'is_editable',
    key: 'is_editable',
    width: 80,
    customRender: ({ text }: { text: boolean }) => (text ? '是' : '否'),
  },
  { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 180 },
];
</script>
<template>
  <ResourcePage
    title="系统配置"
    :columns="columns"
    :search-fields="[
      { key: 'keyword', label: '关键词' },
      { key: 'config_group', label: '配置分组' },
    ]"
    component-name="system-config-detail"
    :fetch-api="getSystemConfigList"
    :create-api="createSystemConfig"
    :update-api="updateSystemConfig"
    :delete-api="deleteSystemConfig"
  />
</template>
