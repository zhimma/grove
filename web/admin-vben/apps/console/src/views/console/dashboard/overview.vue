<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';

import { Card, Col, List, Row, Statistic, Tag } from 'ant-design-vue';

import { getDashboardOverview } from '#/api/core/console';

defineOptions({ name: 'ConsoleDashboardOverview' });

const overview = ref<Record<string, any>>({});

const statItems = computed(() => [
  { label: '管理员总数', value: overview.value.admin_count || 0 },
  { label: '角色总数', value: overview.value.role_count || 0 },
  { label: '操作日志', value: overview.value.operation_count || 0 },
  { label: '登录日志', value: overview.value.login_count || 0 },
]);

async function loadDashboard() {
  overview.value = await getDashboardOverview();
}

onMounted(loadDashboard);
</script>

<template>
  <div class="p-5">
    <Row :gutter="[16, 16]">
      <Col :span="6">
        <Card>
          <Statistic title="管理员总数" :value="overview.admin_count || 0" />
        </Card>
      </Col>
      <Col :span="6">
        <Card>
          <Statistic title="角色总数" :value="overview.role_count || 0" />
        </Card>
      </Col>
      <Col :span="6">
        <Card>
          <Statistic title="操作日志" :value="overview.operation_count || 0" />
        </Card>
      </Col>
      <Col :span="6">
        <Card>
          <Statistic title="登录日志" :value="overview.login_count || 0" />
        </Card>
      </Col>
    </Row>

    <Row :gutter="[16, 16]" class="mt-4">
      <Col :span="12">
        <Card title="模板概览">
          <List :data-source="statItems">
            <template #renderItem="{ item }">
              <List.Item>
                <span>{{ item.label }}</span>
                <Tag color="processing">{{ item.value }}</Tag>
              </List.Item>
            </template>
          </List>
        </Card>
      </Col>
      <Col :span="12">
        <Card title="运行状态">
          <div class="space-y-3">
            <div class="text-sm text-gray-500">当前状态消息</div>
            <Tag color="blue">{{ overview.message || 'admin template ready' }}</Tag>
            <div class="text-sm text-gray-500">
              当前后台已收缩为通用模板，仅保留登录鉴权、工作台、系统配置、管理员与角色权限。
            </div>
          </div>
        </Card>
      </Col>
    </Row>
  </div>
</template>
