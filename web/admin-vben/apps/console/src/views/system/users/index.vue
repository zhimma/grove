<script setup lang="ts">
import {
  createUser,
  deleteUser,
  getUserList,
  updateUser,
  updateUserStatus,
} from '#/api/user';
import ResourcePage from '#/components/resource-page/index.vue';

const statusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
];

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name', width: 160 },
  { title: '邮箱', dataIndex: 'email', key: 'email', width: 220 },
  { title: '手机号', dataIndex: 'phone', key: 'phone', width: 150 },
  { title: '状态', dataIndex: 'status_text', key: 'status', width: 100 },
  {
    title: '最后登录',
    dataIndex: 'last_login_at',
    key: 'last_login_at',
    width: 180,
  },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
];

function normalizePayload(payload: Record<string, any>) {
  const result = { ...payload };
  if (!result.status && result.status !== 0) {
    delete result.status;
  }
  return result;
}
</script>

<template>
  <ResourcePage
    title="用户管理"
    :columns="columns"
    :search-fields="[
      { key: 'keyword', label: '关键词' },
      { key: 'status', label: '状态', type: 'select', options: statusOptions },
    ]"
    :form-fields="[
      { key: 'name', label: '名称', required: true },
      { key: 'email', label: '邮箱', required: true },
      { key: 'phone', label: '手机号' },
      {
        key: 'avatar',
        label: '头像',
        type: 'uploadImg',
        purpose: 'avatar',
        maxCount: 1,
      },
      { key: 'remark', label: '备注', type: 'textarea' },
      {
        key: 'status',
        label: '状态',
        type: 'radio',
        options: statusOptions,
      },
    ]"
    :fetch-api="getUserList"
    :create-api="createUser"
    :update-api="updateUser"
    :delete-api="deleteUser"
    :status-api="updateUserStatus"
    :transform-payload="normalizePayload"
  />
</template>
