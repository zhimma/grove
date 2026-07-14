import type { useAccessStore } from '@vben/stores';

import type { usePermissionStore } from './permission';

type ConsoleAccessStore = Pick<
  ReturnType<typeof useAccessStore>,
  | 'setAccessCodes'
  | 'setAccessMenus'
  | 'setAccessRoutes'
  | 'setAccessToken'
  | 'setIsAccessChecked'
  | 'setLoginExpired'
  | 'setRefreshToken'
>;

type ConsolePermissionStore = Pick<
  ReturnType<typeof usePermissionStore>,
  '$reset'
>;

export function clearConsoleAuthState(
  accessStore: ConsoleAccessStore,
  permissionStore: ConsolePermissionStore,
) {
  accessStore.setAccessToken(null);
  accessStore.setRefreshToken(null);
  accessStore.setAccessCodes([]);
  accessStore.setAccessMenus([]);
  accessStore.setAccessRoutes([]);
  accessStore.setIsAccessChecked(false);
  accessStore.setLoginExpired(false);
  permissionStore.$reset();
}
