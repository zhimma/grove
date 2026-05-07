<script setup lang="ts">
import { ref } from 'vue';

interface Props {
  visible: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
});

const emit = defineEmits<{
  'update:visible': [value: boolean];
  submit: [data: { old_password: string; new_password: string }];
}>();

// 表单数据
const formRef = ref();
const formData = ref({
  old_password: '',
  new_password: '',
  confirm_password: '',
});

// 加载状态
const loading = ref(false);

// 关闭弹窗
function handleCancel() {
  emit('update:visible', false);
  formRef.value?.resetFields();
}

// 确认密码验证
function validateConfirmPassword(_rule: any, value: string) {
  if (value !== formData.value.new_password) {
    return Promise.reject('两次输入的密码不一致');
  }
  return Promise.resolve();
}

// 提交表单
async function handleSubmit() {
  try {
    await formRef.value?.validate();
    emit('submit', {
      old_password: formData.value.old_password,
      new_password: formData.value.new_password,
    });
  } catch (error) {
    // 表单验证错误
  }
}
</script>

<template>
  <a-modal
    :visible="props.visible"
    title="修改密码"
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
        label="当前密码"
        name="old_password"
        :rules="[
          { required: true, message: '请输入当前密码' },
        ]"
      >
        <a-input-password
          v-model:value="formData.old_password"
          placeholder="请输入当前密码"
        />
      </a-form-item>

      <a-form-item
        label="新密码"
        name="new_password"
        :rules="[
          { required: true, message: '请输入新密码' },
          { min: 6, message: '密码长度不能少于6位' },
        ]"
      >
        <a-input-password
          v-model:value="formData.new_password"
          placeholder="请输入新密码"
        />
      </a-form-item>

      <a-form-item
        label="确认新密码"
        name="confirm_password"
        :rules="[
          { required: true, message: '请确认新密码' },
          { validator: validateConfirmPassword },
        ]"
      >
        <a-input-password
          v-model:value="formData.confirm_password"
          placeholder="请再次输入新密码"
        />
      </a-form-item>
    </a-form>
  </a-modal>
</template>
