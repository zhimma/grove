import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { usePermissionStore } from './permission';

const mocks = vi.hoisted(() => ({
  getAuthorizationOverviewApi: vi.fn(),
}));

vi.mock('#/api/auth', () => ({
  getAuthorizationOverviewApi: mocks.getAuthorizationOverviewApi,
}));

describe('permission store', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    mocks.getAuthorizationOverviewApi.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fails closed before authorization has loaded', () => {
    const store = usePermissionStore();

    expect(store.hasPermission('GET /console/v1/roles')).toBe(false);
    expect(store.hasMenuAccess('ConsoleRoles')).toBe(false);
  });

  it('loads exact and wildcard permissions', async () => {
    mocks.getAuthorizationOverviewApi.mockResolvedValue({
      api_permissions: ['GET /console/v1/roles'],
      menu_keys: ['ConsoleRoles'],
    });
    const store = usePermissionStore();

    await store.initPermissions();

    expect(store.isLoaded).toBe(true);
    expect(store.hasApiPermission(' get ', ' /console/v1/roles ')).toBe(true);
    expect(store.hasApiPermission('POST', '/console/v1/roles')).toBe(false);
    expect(store.hasMenuAccess('ConsoleRoles')).toBe(true);
    expect(store.hasMenuAccess('ConsoleAdmins')).toBe(false);

    store.apiPermissions = ['*'];
    store.menuKeys = ['*'];
    expect(store.hasPermission('DELETE /anything')).toBe(true);
    expect(store.hasMenuAccess('AnyMenu')).toBe(true);
  });

  it('stays fail closed when authorization loading fails', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
    mocks.getAuthorizationOverviewApi.mockRejectedValue(
      new Error('authorization unavailable'),
    );
    const store = usePermissionStore();

    await store.initPermissions();

    expect(store.isLoaded).toBe(true);
    expect(store.apiPermissions).toEqual([]);
    expect(store.menuKeys).toEqual([]);
    expect(store.hasPermission('GET /console/v1/roles')).toBe(false);
    expect(store.hasMenuAccess('ConsoleRoles')).toBe(false);
  });

  it('resets all permission state', async () => {
    mocks.getAuthorizationOverviewApi.mockResolvedValue({
      api_permissions: ['*'],
      menu_keys: ['*'],
    });
    const store = usePermissionStore();
    await store.initPermissions();

    store.$reset();

    expect(store.apiPermissions).toEqual([]);
    expect(store.menuKeys).toEqual([]);
    expect(store.isLoaded).toBe(false);
    expect(store.hasPermission('GET /console/v1/roles')).toBe(false);
  });
});
