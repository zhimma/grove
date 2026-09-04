import type { RouteRecordRaw } from 'vue-router';

// Route modules are concatenated, not merged by path, so every /system child
// has to live in this one array. Declaring some of them in another module put
// them outside the 系统管理 group and rendered them as top-level menu items.
const routes: RouteRecordRaw[] = [
  {
    meta: { icon: 'lucide:settings', order: 9999, title: '系统管理' },
    name: 'ConsoleSystem',
    path: '/system',
    children: [
      {
        name: 'ConsoleAdmins',
        path: '/system/admins',
        component: () => import('#/views/system/admins/index.vue'),
        meta: { title: '管理员管理' },
      },
      {
        name: 'ConsoleUsers',
        path: '/system/users',
        component: () => import('#/views/system/users/index.vue'),
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
        component: () => import('#/views/system/sessions/index.vue'),
        meta: {
          title: '在线会话',
          permissions: ['系统管理.会话列表'],
        },
      },
      {
        name: 'ConsoleScheduledTasks',
        path: '/system/scheduled-task',
        component: () => import('#/views/system/scheduled-task/index.vue'),
        meta: {
          icon: 'lucide:clock',
          title: '计划任务',
          permissions: ['计划任务.任务列表'],
        },
      },
      {
        name: 'ConsoleOperationLog',
        path: '/system/operation-log',
        component: () => import('#/views/system/operation-log/index.vue'),
        meta: {
          icon: 'lucide:file-text',
          title: '操作日志',
          permissions: ['系统日志.操作日志列表'],
        },
      },
      {
        name: 'ConsoleLoginLog',
        path: '/system/login-log',
        component: () => import('#/views/system/login-log/index.vue'),
        meta: {
          icon: 'lucide:log-in',
          title: '登录日志',
          permissions: ['系统日志.登录日志列表'],
        },
      },
    ],
  },
];

export default routes;
