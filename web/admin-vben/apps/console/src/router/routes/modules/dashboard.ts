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
        component: () => import('#/views/dashboard/overview/index.vue'),
        meta: { affixTab: true, title: '工作台' },
      },
    ],
  },
];

export default routes;
