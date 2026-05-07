import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    meta: {
      icon: 'lucide:layout-dashboard',
      order: -1,
      title: '工作台',
    },
    name: 'ConsoleDashboard',
    path: '/dashboard',
    redirect: '/dashboard/overview',
    children: [
      {
        name: 'ConsoleOverview',
        path: '/dashboard/overview',
        component: () => import('#/views/console/dashboard/overview.vue'),
        meta: { affixTab: true, title: '工作台' },
      },
    ],
  },
  {
    meta: { icon: 'lucide:settings-2', order: 1010, title: '配置管理' },
    name: 'ConsoleConfigs',
    path: '/configs',
    children: [
      {
        name: 'ConsoleSystemConfigs',
        path: '/configs/system',
        component: () => import('#/views/console/configs/system-configs.vue'),
        meta: { title: '系统配置' },
      },
    ],
  },
  {
    meta: { icon: 'lucide:settings', order: 9999, title: '系统管理' },
    name: 'ConsoleSystem',
    path: '/system',
    children: [
      {
        name: 'ConsoleAdmins',
        path: '/system/admins',
        component: () => import('#/views/console/system/admins.vue'),
        meta: { title: '管理员管理' },
      },
      {
        name: 'ConsoleRoles',
        path: '/system/role',
        component: () => import('#/views/system/role/index.vue'),
        meta: { title: '角色权限' },
      },
    ],
  },
];

export default routes;
