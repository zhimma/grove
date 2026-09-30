import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    meta: { icon: 'lucide:notebook-tabs', order: 1005, title: '内容管理' },
    name: 'ConsoleContent',
    path: '/content',
    children: [
      {
        name: 'ConsoleArticles',
        path: '/content/articles',
        component: () => import('#/views/content/articles/index.vue'),
        meta: { title: '文章管理' },
      },
    ],
  },
];

export default routes;
