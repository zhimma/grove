<script setup lang="ts">
import { ref, watch } from 'vue';

interface Props {
  visible: boolean;
  initialValues?: {
    email?: string;
    phone?: string;
    avatar?: string;
  } | null;
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  initialValues: null,
});

const emit = defineEmits<{
  'update:visible': [value: boolean];
  submit: [data: { email?: string; phone?: string; avatar?: string }];
}>();

// 表单数据
const formRef = ref();
const formData = ref({
  email: '',
  phone: '',
  avatar: '',
});

// 加载状态
const loading = ref(false);

// 监听初始值
watch(
  () => props.initialValues,
  (val) => {
    if (val) {
      formData.value = {
        email: val.email || '',
        phone: val.phone || '',
        avatar: val.avatar || '',
      };
    }
  },
  { immediate: true }
);

// 关闭弹窗
function handleCancel() {
  emit('update:visible', false);
  formRef.value?.resetFields();
}

// 提交表单
async function handleSubmit() {
  try {
    await formRef.value?.validate();
    emit('submit', { ...formData.value });
  } catch (error) {
    // 表单验证错误
  }
}
</script>

<template>
  <a-modal
    :visible="props.visible"
    title="编辑个人资料"
    :confirm-loading="loading"
    @ok="handleSubmit"
    @cancel="handleCancel"
  >
    <a-form
      ref="formRef"
      :model="formData"
      layout="vertical"
    >
      <a-form-item
        label="邮箱"
        name="email"
        :rules="[
          { type: 'email', message: '请输入有效的邮箱地址' },
        ]"
      >
        <a-input
          v-model:value="formData.email"
          placeholder="请输入邮箱"
        />
      </a-form-item>

      <a-form-item
        label="手机号"
        name="phone"
        :rules="[
          { pattern: /^1[3-9]\d{9}$/, message: '请输入有效的手机号' },
        ]"
      >
        <a-input
          v-model:value="formData.phone"
          placeholder="请输入手机号"
        />
      </a-form-item>

      <a-form-item
        label="头像URL"
        name="avatar"
      >
        <a-input
          v-model:value="formData.avatar"
          placeholder="请输入头像URL"
        />
      </a-form-item>
    </a-form>
  </a-modal>
</template>
