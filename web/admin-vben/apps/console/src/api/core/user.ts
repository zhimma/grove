import type { UserInfo } from '@vben/types';

import { requestClient } from '#/api/request';

/**
 * 获取用户信息
 */
export async function getUserInfoApi() {
  const data = await requestClient.get<Record<string, any>>(
    '/console/v1/auth/me',
  );
  return {
    account: data.account || '',
    avatar: data.avatar,
    desc: data.role?.display_name || data.role?.name || '平台管理员',
    email: data.email || '',
    homePath: '/dashboard/overview',
    nickname:
      data.display_name ||
      data.real_name ||
      data.username ||
      data.account ||
      '',
    phone: data.phone || '',
    realName:
      data.display_name ||
      data.real_name ||
      data.username ||
      data.account ||
      data.phone,
    real_name: data.real_name || '',
    roles: data.role?.name ? [data.role.name] : [],
    token: '',
    userId: data.id,
    username: data.username || '',
  } as UserInfo;
}

export function updateCurrentUserApi(data: Record<string, any>) {
  return requestClient.put<Record<string, any>>('/console/v1/auth/me', data);
}

export function changePasswordApi(data: {
  new_password: string;
  old_password: string;
}) {
  return requestClient.put('/console/v1/auth/password', data);
}
