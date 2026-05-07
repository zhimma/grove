<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { message } from 'ant-design-vue';
import { getProfile, updateProfile, changePassword } from '#/api/profile';
import type { Profile, UpdateProfileParams, ChangePasswordParams } from '#/api/profile';
import ProfileForm from './components/ProfileForm.vue';
import PasswordForm from './components/PasswordForm.vue';

// 状态
const loading = ref(false);
const profile = ref<Profile | null>(null);

// 表单弹窗
const profileFormVisible = ref(false);
const passwordFormVisible = ref(false);

// 加载个人资料
async function loadProfile() {
  loading.value = true;
  try {
    const res = await getProfile();
    profile.value = res;
  } finally {
    loading.value = false;
  }
}

// 更新个人资料
async function handleUpdateProfile(data: UpdateProfileParams) {
  try {
    const res = await updateProfile(data);
    profile.value = res;
    message.success('更新成功');
    profileFormVisible.value = false;
  } catch (error) {
    // 错误已在拦截器处理
  }
}

// 修改密码
async function handleChangePassword(data: ChangePasswordParams) {
  try {
    await changePassword(data);
    message.success('密码修改成功，请重新登录');
    passwordFormVisible.value = false;
  } catch (error) {
    // 错误已在拦截器处理
  }
}

onMounted(() => {
  loadProfile();
});
</script>

<template>
  <div class="profile-page">
    <a-row :gutter="24">
      <!-- 左侧：个人信息卡片 -->
      <a-col :span="8">
        <a-card :loading="loading">
          <div class="text-center">
            <a-avatar
              :size="100"
              :src="profile?.avatar"
              :alt="profile?.username"
            >
              {{ profile?.username?.charAt(0)?.toUpperCase() }}
            </a-avatar>
            <h2 class="mt-4 mb-2">{{ profile?.username }}</h2>
            <p class="text-gray-500">{{ profile?.role_name }}</p>
            <a-tag v-if="profile?.is_super_admin" color="red">超级管理员</a-tag>
          </div>

          <a-divider />

          <div class="info-list">
            <div class="info-item">
              <span class="label">邮箱：</span>
              <span class="value">{{ profile?.email || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">电话：</span>
              <span class="value">{{ profile?.phone || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">状态：</span>
              <a-tag :color="profile?.status === 1 ? 'success' : 'error'">
                {{ profile?.status === 1 ? '正常' : '禁用' }}
              </a-tag>
            </div>
            <div class="info-item">
              <span class="label">创建时间：</span>
              <span class="value">{{ profile?.created_at }}</span>
            </div>
          </div>

          <a-divider />

          <div class="action-buttons">
            <a-button type="primary" block @click="profileFormVisible = true">
              编辑资料
            </a-button>
            <a-button class="mt-2" block @click="passwordFormVisible = true">
              修改密码
            </a-button>
          </div>
        </a-card>
      </a-col>

      <!-- 右侧：详细信息 -->
      <a-col :span="16">
        <a-card title="基本信息">
          <a-descriptions :column="2">
            <a-descriptions-item label="用户ID">
              {{ profile?.id }}
            </a-descriptions-item>
            <a-descriptions-item label="用户名">
              {{ profile?.username }}
            </a-descriptions-item>
            <a-descriptions-item label="角色">
              {{ profile?.role_name }}
            </a-descriptions-item>
            <a-descriptions-item label="角色ID">
              {{ profile?.role_id }}
            </a-descriptions-item>
            <a-descriptions-item label="邮箱">
              {{ profile?.email || '-' }}
            </a-descriptions-item>
            <a-descriptions-item label="电话">
              {{ profile?.phone || '-' }}
            </a-descriptions-item>
          </a-descriptions>
        </a-card>

        <a-card title="安全设置" class="mt-4">
          <a-list>
            <a-list-item>
              <a-list-item-meta
                title="账户密码"
                description="定期修改密码有助于保护账户安全"
              />
              <template #actions>
                <a-button type="link" @click="passwordFormVisible = true">
                  修改
                </a-button>
              </template>
            </a-list-item>
            <a-list-item>
              <a-list-item-meta
                title="绑定手机"
                :description="profile?.phone ? '已绑定：' + profile.phone : '未绑定手机号'"
              />
              <template #actions>
                <a-button type="link" @click="profileFormVisible = true">
                  {{ profile?.phone ? '修改' : '绑定' }}
                </a-button>
              </template>
            </a-list-item>
            <a-list-item>
              <a-list-item-meta
                title="绑定邮箱"
                :description="profile?.email ? '已绑定：' + profile.email : '未绑定邮箱'"
              />
              <template #actions>
                <a-button type="link" @click="profileFormVisible = true">
                  {{ profile?.email ? '修改' : '绑定' }}
                </a-button>
              </template>
            </a-list-item>
          </a-list>
        </a-card>
      </a-col>
    </a-row>

    <!-- 编辑资料弹窗 -->
    <ProfileForm
      v-model:visible="profileFormVisible"
      :initial-values="profile"
      @submit="handleUpdateProfile"
    />

    <!-- 修改密码弹窗 -->
    <PasswordForm
      v-model:visible="passwordFormVisible"
      @submit="handleChangePassword"
    />
  </div>
</template>

<style scoped>
.profile-page {
  padding: 24px;
}

.info-list {
  .info-item {
    display: flex;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid #f0f0f0;

    &:last-child {
      border-bottom: none;
    }

    .label {
      color: #666;
    }

    .value {
      color: #333;
      font-weight: 500;
    }
  }
}

.action-buttons {
  margin-top: 16px;
}

.mt-2 {
  margin-top: 8px;
}

.mt-4 {
  margin-top: 16px;
}

.mb-2 {
  margin-bottom: 8px;
}
</style>
