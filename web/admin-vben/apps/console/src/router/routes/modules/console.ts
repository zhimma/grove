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
      {
        name: 'ConsoleSiteConfigs',
        path: '/configs/site',
        component: () => import('#/views/console/configs/site-configs.vue'),
        meta: { title: '站点配置' },
      },
    ],
  },
  {
    meta: { icon: 'lucide:notebook-tabs', order: 1005, title: '内容管理' },
    name: 'ConsoleContent',
    path: '/content',
    children: [
      {
        name: 'ConsoleArticles',
        path: '/content/articles',
        component: () => import('#/views/console/content/articles.vue'),
        meta: { title: '文章管理' },
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
        name: 'ConsoleUsers',
        path: '/system/users',
        component: () => import('#/views/console/system/users.vue'),
        meta: { title: '用户管理' },
      },
      {
        name: 'ConsoleRoles',
        path: '/system/role',
        component: () => import('#/views/system/role/index.vue'),
        meta: { title: '角色权限' },
      },
      {
        name: 'ConsoleSessions',
        path: '/system/sessions',
        component: () => import('#/views/console/system/sessions.vue'),
        meta: {
          title: '在线会话',
          permissions: ['系统管理.会话列表'],
        },
      },
    ],
  },
];

export default routes;
