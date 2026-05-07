import { requestClient } from '#/api/request';

export interface Profile {
  id: string;
  username: string;
  email: string;
  phone: string;
  avatar: string;
  role_id: string;
  role_name: string;
  is_super_admin: boolean;
  status: number;
  created_at: string;
}

export interface UpdateProfileParams {
  email?: string;
  phone?: string;
  avatar?: string;
}

export interface ChangePasswordParams {
  old_password: string;
  new_password: string;
}

export interface TokenInfo {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

// 获取个人资料
export function getProfile() {
  return requestClient.get<Profile>('/console/v1/profile');
}

// 更新个人资料
export function updateProfile(data: UpdateProfileParams) {
  return requestClient.put<Profile>('/console/v1/profile', data);
}

// 修改密码
export function changePassword(data: ChangePasswordParams) {
  return requestClient.put('/console/v1/profile/password', data);
}

// 刷新Token
export function refreshToken(refreshToken: string) {
  return requestClient.post<TokenInfo>('/console/v1/profile/refresh-token', {
    refresh_token: refreshToken,
  });
}
