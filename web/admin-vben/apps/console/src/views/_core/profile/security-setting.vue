<script setup lang="ts">
import { computed } from 'vue';

import { ProfileSecuritySetting } from '@vben/common-ui';
import { useUserStore } from '@vben/stores';

const userStore = useUserStore();

const formSchema = computed(() => {
  const user = userStore.userInfo as Record<string, any>;
  return [
    {
      value: true,
      fieldName: 'accountPassword',
      label: '账户密码',
      description: '建议定期更换登录密码',
    },
    {
      value: !!user?.phone,
      fieldName: 'securityPhone',
      label: '登录手机',
      description: user?.phone || '未设置手机号',
    },
    {
      value: !!user?.email,
      fieldName: 'securityEmail',
      label: '登录邮箱',
      description: user?.email || '未设置邮箱',
    },
    {
      value: !!user?.account,
      fieldName: 'securityUsername',
      label: '登录账号',
      description: user?.account || '未设置账号',
    },
  ];
});
</script>
<template>
  <ProfileSecuritySetting :form-schema="formSchema" />
</template>
