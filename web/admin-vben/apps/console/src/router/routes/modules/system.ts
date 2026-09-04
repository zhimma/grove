import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    path: '/system/scheduled-task',
    name: 'system.scheduled-task',
    component: () => import('#/views/system/scheduled-task/index.vue'),
    meta: {
      title: '计划任务',
      icon: 'ClockCircleOutlined',
      permissions: ['计划任务.任务列表'],
    },
  },
];

export default routes;
