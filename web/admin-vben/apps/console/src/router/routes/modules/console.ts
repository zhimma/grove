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
];

export default routes;
