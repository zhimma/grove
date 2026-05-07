import { ref } from 'vue';

import { defineStore } from 'pinia';

import { getAuthorizationOverviewApi } from '#/api/core/auth';

export const usePermissionStore = defineStore('permission', () => {
  const apiPermissions = ref<string[]>([]);
  const menuKeys = ref<string[]>([]);
  const isLoaded = ref(false);

  async function loadAuthorizationOverview() {
    try {
      const data = await getAuthorizationOverviewApi();
      apiPermissions.value = data.api_permissions || [];
      menuKeys.value = data.menu_keys || [];
      return data;
    } catch (error) {
      console.error('加载授权总览失败:', error);
      apiPermissions.value = [];
      menuKeys.value = [];
      return { api_permissions: [], menu_keys: [] };
    }
  }

  function hasPermission(permission: string): boolean {
    if (!isLoaded.value) {
      return true;
    }

    if (apiPermissions.value.includes('*')) {
      return true;
    }

    return apiPermissions.value.includes(permission);
  }

  function hasApiPermission(method: string, path: string): boolean {
    return hasPermission(buildApiPermissionIdentifier(method, path));
  }

  function hasMenuAccess(menuKey: string): boolean {
    if (!isLoaded.value) {
      return true;
    }

    if (menuKeys.value.includes('*')) {
      return true;
    }

    return menuKeys.value.includes(menuKey);
  }

  async function initPermissions() {
    await loadAuthorizationOverview();
    isLoaded.value = true;
  }

  function $reset() {
    apiPermissions.value = [];
    menuKeys.value = [];
    isLoaded.value = false;
  }

  return {
    apiPermissions,
    hasApiPermission,
    hasMenuAccess,
    hasPermission,
    initPermissions,
    isLoaded,
    loadAuthorizationOverview,
    menuKeys,
    $reset,
  };
});

function buildApiPermissionIdentifier(method: string, path: string): string {
  return `${method.toUpperCase().trim()} ${path.trim()}`;
}
