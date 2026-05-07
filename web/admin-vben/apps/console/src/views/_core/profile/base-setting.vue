<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';

import { Button, Card, Form, Input, message } from 'ant-design-vue';

import { getUserInfoApi, updateCurrentUserApi } from '#/api';

const loading = ref(false);
const formRef = ref();
const form = reactive({
  account: '',
  email: '',
  nickname: '',
  phone: '',
  real_name: '',
  username: '',
});

async function loadProfile() {
  const data = await getUserInfoApi();
  Object.assign(form, {
    account: (data as any).account || '',
    email: (data as any).email || '',
    nickname: (data as any).nickname || '',
    phone: (data as any).phone || '',
    real_name: (data as any).real_name || '',
    username: data.username || '',
  });
}

async function handleSubmit() {
  const valid = await formRef.value?.validate();
  if (!valid) return;

  loading.value = true;
  try {
    await updateCurrentUserApi(form);
    message.success('管理员资料已保存');
    await loadProfile();
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  loadProfile();
});
</script>

<template>
  <Card title="管理员资料">
    <Form ref="formRef" :model="form" layout="vertical">
      <Form.Item
        label="登录账号"
        name="account"
        :rules="[{ required: true, message: '请输入登录账号' }]"
      >
        <Input v-model:value="form.account" />
      </Form.Item>
      <Form.Item label="用户名" name="username">
        <Input v-model:value="form.username" />
      </Form.Item>
      <Form.Item label="真实姓名" name="real_name">
        <Input v-model:value="form.real_name" />
      </Form.Item>
      <Form.Item label="昵称" name="nickname">
        <Input v-model:value="form.nickname" />
      </Form.Item>
      <Form.Item label="手机号" name="phone">
        <Input v-model:value="form.phone" />
      </Form.Item>
      <Form.Item label="邮箱" name="email">
        <Input v-model:value="form.email" />
      </Form.Item>
      <Button type="primary" :loading="loading" @click="handleSubmit">
        保存管理员资料
      </Button>
    </Form>
  </Card>
</template>
