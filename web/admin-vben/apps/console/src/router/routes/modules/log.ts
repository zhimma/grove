import type { RouteRecordRaw } from 'vue-router';

const logRoutes: RouteRecordRaw[] = [
  {
    path: '/system/operation-log',
    name: 'system.operation-log',
    component: () => import('#/views/system/operation-log/index.vue'),
    meta: {
      title: '操作日志',
      icon: 'FileTextOutlined',
      permissions: ['系统日志.操作日志列表'],
    },
  },
  {
    path: '/system/login-log',
    name: 'system.login-log',
    component: () => import('#/views/system/login-log/index.vue'),
    meta: {
      title: '登录日志',
      icon: 'LoginOutlined',
      permissions: ['系统日志.登录日志列表'],
    },
  },
];

export default logRoutes;
