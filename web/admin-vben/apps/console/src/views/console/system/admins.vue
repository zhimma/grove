<script setup lang="ts">
import {
  createAdmin,
  deleteAdmin,
  getAdminList,
  updateAdmin,
  updateAdminStatus,
} from '#/api/core/console';

import ResourcePage from '../shared/resource-page.vue';

const columns = [
  { title: '账号', dataIndex: 'account', key: 'account', width: 160 },
  { title: '姓名', dataIndex: 'display_name', key: 'display_name', width: 140 },
  { title: '手机号', dataIndex: 'phone', key: 'phone', width: 140 },
  { title: '邮箱', dataIndex: 'email', key: 'email', width: 200 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
];
</script>
<template>
  <ResourcePage
    title="管理员管理"
    :columns="columns"
    :search-fields="[
      { key: 'keyword', label: '关键词' },
      { key: 'status', label: '状态' },
    ]"
    :form-fields="[
      { key: 'account', label: '账号', required: true },
      { key: 'username', label: '用户名' },
      { key: 'display_name', label: '显示名' },
      { key: 'phone', label: '手机号' },
      { key: 'email', label: '邮箱' },
      { key: 'password', label: '密码' },
      { key: 'role_id', label: '角色ID' },
    ]"
    :fetch-api="getAdminList"
    :create-api="createAdmin"
    :update-api="updateAdmin"
    :delete-api="deleteAdmin"
    :status-api="updateAdminStatus"
  />
</template>
