import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useAuthStore } from './auth';

const mocks = vi.hoisted(() => {
  const accessStore = {
    accessToken: 'access-token' as null | string,
    loginExpired: true,
    refreshToken: 'refresh-token' as null | string,
    setAccessCodes: vi.fn(),
    setAccessMenus: vi.fn(),
    setAccessRoutes: vi.fn(),
    setAccessToken: vi.fn(),
    setIsAccessChecked: vi.fn(),
    setLoginExpired: vi.fn(),
    setRefreshToken: vi.fn(),
  };
  return {
    accessStore,
    logoutApi: vi.fn(),
    permissionStore: { $reset: vi.fn(), initPermissions: vi.fn() },
    resetAllStores: vi.fn(),
    router: {
      currentRoute: { value: { fullPath: '/roles?page=2' } },
      push: vi.fn(),
      replace: vi.fn(),
    },
    userStore: { setUserInfo: vi.fn() },
  };
});

vi.mock('vue-router', () => ({ useRouter: () => mocks.router }));
vi.mock('@vben/constants', () => ({ LOGIN_PATH: '/login' }));
vi.mock('@vben/preferences', () => ({
  preferences: { app: { defaultHomePath: '/', loginExpiredMode: 'page' } },
}));
vi.mock('@vben/stores', () => ({
  resetAllStores: mocks.resetAllStores,
  useAccessStore: () => mocks.accessStore,
  useUserStore: () => mocks.userStore,
}));
vi.mock('ant-design-vue', () => ({
  notification: { success: vi.fn() },
}));
vi.mock('#/api', () => ({
  getAccessCodesApi: vi.fn(),
  getUserInfoApi: vi.fn(),
  loginApi: vi.fn(),
  logoutApi: mocks.logoutApi,
}));
vi.mock('#/locales', () => ({ $t: (key: string) => key }));
vi.mock('./permission', () => ({
  usePermissionStore: () => mocks.permissionStore,
}));

describe('auth store logout', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    mocks.accessStore.accessToken = 'access-token';
    mocks.accessStore.refreshToken = 'refresh-token';
    mocks.logoutApi.mockReset().mockResolvedValue(undefined);
    mocks.resetAllStores.mockReset();
    mocks.permissionStore.$reset.mockReset();
    mocks.router.replace.mockReset().mockResolvedValue(undefined);
    Object.values(mocks.accessStore).forEach((value) => {
      if (typeof value === 'function' && 'mockClear' in value) {
        value.mockClear();
      }
    });
  });

  it('notifies the server and clears all console authorization state', async () => {
    const store = useAuthStore();

    await store.logout();

    expect(mocks.logoutApi).toHaveBeenCalledWith('refresh-token');
    expect(mocks.resetAllStores).toHaveBeenCalledOnce();
    expect(mocks.accessStore.setAccessToken).toHaveBeenCalledWith(null);
    expect(mocks.accessStore.setRefreshToken).toHaveBeenCalledWith(null);
    expect(mocks.accessStore.setAccessCodes).toHaveBeenCalledWith([]);
    expect(mocks.accessStore.setAccessMenus).toHaveBeenCalledWith([]);
    expect(mocks.accessStore.setAccessRoutes).toHaveBeenCalledWith([]);
    expect(mocks.accessStore.setIsAccessChecked).toHaveBeenCalledWith(false);
    expect(mocks.accessStore.setLoginExpired).toHaveBeenCalledWith(false);
    expect(mocks.permissionStore.$reset).toHaveBeenCalledOnce();
    expect(mocks.router.replace).toHaveBeenCalledWith({
      path: '/login',
      query: { redirect: '%2Froles%3Fpage%3D2' },
    });
  });
});
