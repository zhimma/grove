import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    meta: { icon: 'lucide:settings-2', order: 1010, title: '配置管理' },
    name: 'ConsoleConfigs',
    path: '/configs',
    children: [
      {
        name: 'ConsoleSystemConfigs',
        path: '/configs/system',
        component: () => import('#/views/configs/system/index.vue'),
        meta: { title: '系统配置' },
      },
      {
        name: 'ConsoleSiteConfigs',
        path: '/configs/site',
        component: () => import('#/views/configs/site/index.vue'),
        meta: { title: '站点配置' },
      },
    ],
  },
];

export default routes;
