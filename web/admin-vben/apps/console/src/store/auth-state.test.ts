import { describe, expect, it, vi } from 'vitest';

import { clearConsoleAuthState } from './auth-state';

describe('clearConsoleAuthState', () => {
  it('clears tokens, permissions, menus, routes and access flags', () => {
    const accessStore = {
      setAccessCodes: vi.fn(),
      setAccessMenus: vi.fn(),
      setAccessRoutes: vi.fn(),
      setAccessToken: vi.fn(),
      setIsAccessChecked: vi.fn(),
      setLoginExpired: vi.fn(),
      setRefreshToken: vi.fn(),
    };
    const permissionStore = { $reset: vi.fn() };

    clearConsoleAuthState(accessStore, permissionStore);

    expect(accessStore.setAccessToken).toHaveBeenCalledWith(null);
    expect(accessStore.setRefreshToken).toHaveBeenCalledWith(null);
    expect(accessStore.setAccessCodes).toHaveBeenCalledWith([]);
    expect(accessStore.setAccessMenus).toHaveBeenCalledWith([]);
    expect(accessStore.setAccessRoutes).toHaveBeenCalledWith([]);
    expect(accessStore.setIsAccessChecked).toHaveBeenCalledWith(false);
    expect(accessStore.setLoginExpired).toHaveBeenCalledWith(false);
    expect(permissionStore.$reset).toHaveBeenCalledOnce();
  });
});
