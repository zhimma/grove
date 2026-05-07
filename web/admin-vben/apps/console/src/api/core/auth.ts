import { baseRequestClient, requestClient } from '#/api/request';

export namespace AuthApi {
  /** 登录接口参数 */
  export interface LoginParams {
    account?: string;
    password?: string;
  }

  /** 登录接口返回值 */
  export interface LoginResult {
    user: Record<string, any>;
    token: {
      access_token: string;
      expires_in: number;
      refresh_token: string;
      token_type: string;
    };
  }

  export interface RefreshTokenResult {
    token: {
      access_token: string;
      expires_in: number;
      refresh_token: string;
      token_type: string;
    };
  }

  export interface AuthorizationOverview {
    api_permissions: string[];
    menu_keys: string[];
  }
}

/**
 * 登录
 */
export async function loginApi(data: AuthApi.LoginParams) {
  const identifier = data.account?.trim();
  const payload = {
    identifier,
    account: identifier,
    password: data.password,
  };
  return requestClient.post<AuthApi.LoginResult>(
    '/console/v1/auth/login',
    payload,
  );
}

/**
 * 刷新accessToken
 */
export async function refreshTokenApi(refreshToken?: null | string) {
  return requestClient.post<AuthApi.RefreshTokenResult>(
    '/console/v1/auth/refresh',
    {
      refresh_token: refreshToken,
    },
  );
}

/**
 * 退出登录
 */
export async function logoutApi(refreshToken?: null | string) {
  return baseRequestClient.post('/console/v1/auth/logout', {
    refresh_token: refreshToken,
  });
}

/**
 * 获取当前用户授权总览
 */
export async function getAuthorizationOverviewApi() {
  return requestClient.get<AuthApi.AuthorizationOverview>(
    '/console/v1/auth/permissions',
  );
}

/**
 * 获取用户权限码
 */
export async function getAccessCodesApi() {
  const overview = await getAuthorizationOverviewApi();
  return { permissions: overview.api_permissions || [] };
}
