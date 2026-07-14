import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => {
  const accessStore = {
    accessToken: 'expired-access' as null | string,
    isAccessChecked: true,
    loginExpired: false,
    refreshToken: 'refresh-1' as null | string,
    setAccessToken: vi.fn((token: null | string) => {
      accessStore.accessToken = token;
    }),
    setLoginExpired: vi.fn((expired: boolean) => {
      accessStore.loginExpired = expired;
    }),
    setRefreshToken: vi.fn((token: null | string) => {
      accessStore.refreshToken = token;
    }),
  };
  return {
    accessStore,
    logout: vi.fn(),
    refreshTokenApi: vi.fn(),
  };
});

vi.mock('@vben/hooks', () => ({
  useAppConfig: () => ({ apiURL: 'https://api.example.test' }),
}));
vi.mock('@vben/preferences', () => ({
  preferences: {
    app: {
      enableRefreshToken: true,
      locale: 'zh-CN',
      loginExpiredMode: 'page',
    },
  },
}));
vi.mock('@vben/stores', () => ({
  useAccessStore: () => mocks.accessStore,
}));
vi.mock('ant-design-vue', () => ({ message: { error: vi.fn() } }));
vi.mock('#/store', () => ({
  useAuthStore: () => ({ logout: mocks.logout }),
}));
vi.mock('#/utils/http-error', () => ({
  parseApiError: () => ({
    fieldErrors: {},
    message: 'unauthorized',
    status: 401,
  }),
}));
vi.mock('./core', () => ({
  refreshTokenApi: mocks.refreshTokenApi,
}));

import { createRequestClient } from './request';

describe('console request client', () => {
  beforeEach(() => {
    mocks.accessStore.accessToken = 'expired-access';
    mocks.accessStore.refreshToken = 'refresh-1';
    mocks.accessStore.loginExpired = false;
    mocks.accessStore.setAccessToken.mockClear();
    mocks.accessStore.setLoginExpired.mockClear();
    mocks.accessStore.setRefreshToken.mockClear();
    mocks.logout.mockReset();
    mocks.refreshTokenApi.mockReset();
  });

  it('performs one refresh for concurrent 401 responses and replays both requests', async () => {
    let resolveRefresh: (value: unknown) => void = () => {};
    mocks.refreshTokenApi.mockReturnValue(
      new Promise((resolve) => {
        resolveRefresh = resolve;
      }),
    );

    const client = createRequestClient('https://api.example.test', {
      responseReturn: 'data',
    });
    client.instance.defaults.adapter = async (config) => {
      const response = {
        config,
        data: { code: 0, data: config.url },
        headers: {},
        status: 200,
        statusText: 'OK',
      };
      if (config.headers?.Authorization === 'Bearer fresh-access') {
        return response;
      }
      return Promise.reject(
        Object.assign(new Error('expired'), {
          config,
          response: {
            ...response,
            data: { code: 401, message: 'expired' },
            status: 401,
            statusText: 'Unauthorized',
          },
        }),
      );
    };

    const first = client.get<string>('/protected/a');
    const second = client.get<string>('/protected/b');
    await vi.waitFor(() => {
      expect(mocks.refreshTokenApi).toHaveBeenCalledTimes(1);
      expect(client.refreshTokenQueue).toHaveLength(1);
    });

    resolveRefresh({
      token: {
        access_token: 'fresh-access',
        expires_in: 3600,
        refresh_token: 'fresh-refresh',
        token_type: 'Bearer',
      },
    });

    await expect(Promise.all([first, second])).resolves.toEqual([
      '/protected/a',
      '/protected/b',
    ]);
    expect(mocks.refreshTokenApi).toHaveBeenCalledTimes(1);
    expect(mocks.refreshTokenApi).toHaveBeenCalledWith('refresh-1');
    expect(mocks.accessStore.setAccessToken).toHaveBeenCalledWith(
      'fresh-access',
    );
    expect(mocks.accessStore.setRefreshToken).toHaveBeenCalledWith(
      'fresh-refresh',
    );
    expect(mocks.logout).not.toHaveBeenCalled();
  });
});
