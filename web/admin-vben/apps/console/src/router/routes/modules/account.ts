import type { RouteRecordRaw } from 'vue-router';

const accountRoutes: RouteRecordRaw[] = [
  {
    path: '/account/center',
    name: 'account.center',
    component: () => import('#/views/account/center/index.vue'),
    meta: {
      title: '个人中心',
      icon: 'UserOutlined',
      hideInMenu: true,
    },
  },
];

export default accountRoutes;
