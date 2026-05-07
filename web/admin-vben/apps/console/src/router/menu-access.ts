import type { RouteRecordRaw } from 'vue-router';

export interface MenuPermissionNode {
  children?: MenuPermissionNode[];
  key: string;
  title: string;
}

export function filterConsoleRoutesByMenuKeys(
  routes: RouteRecordRaw[],
  menuKeys: string[],
): RouteRecordRaw[] {
  if (menuKeys.includes('*')) {
    return routes;
  }

  const allowed = new Set(menuKeys.filter(Boolean));
  const filteredRoutes: RouteRecordRaw[] = [];
  for (const route of routes) {
    const next = filterRoute(route, allowed);
    if (next) {
      filteredRoutes.push(next);
    }
  }
  return filteredRoutes;
}

export function buildConsoleMenuPermissionTree(
  routes: RouteRecordRaw[],
): MenuPermissionNode[] {
  return routes.flatMap((route) => buildMenuNode(route));
}

export function findFirstConsoleAccessiblePath(
  routes: RouteRecordRaw[],
): null | string {
  for (const route of routes) {
    const childPath = Array.isArray(route.children)
      ? findFirstConsoleAccessiblePath(route.children)
      : null;
    if (childPath) {
      return childPath;
    }

    if (typeof route.redirect === 'string' && route.redirect) {
      return route.redirect;
    }

    if (typeof route.path === 'string' && route.path) {
      return route.path;
    }
  }

  return null;
}

function filterRoute(
  route: RouteRecordRaw,
  allowed: Set<string>,
): null | RouteRecordRaw {
  const children = Array.isArray(route.children)
    ? route.children
        .map((child) => filterRoute(child, allowed))
        .filter((item): item is RouteRecordRaw => !!item)
    : [];

  const routeName = typeof route.name === 'string' ? route.name : '';
  const includeSelf = routeName ? allowed.has(routeName) : false;

  if (!includeSelf && children.length === 0) {
    return null;
  }

  const nextRoute = { ...route } as RouteRecordRaw;
  if (children.length > 0) {
    nextRoute.children = children;
  } else {
    delete nextRoute.children;
  }
  return nextRoute;
}

function buildMenuNode(route: RouteRecordRaw): MenuPermissionNode[] {
  const children = Array.isArray(route.children)
    ? route.children.flatMap((item) => buildMenuNode(item))
    : [];

  const routeName = typeof route.name === 'string' ? route.name : '';
  const title = typeof route.meta?.title === 'string' ? route.meta.title : '';
  if (!routeName || !title || route.meta?.hideInMenu) {
    return children;
  }

  return [
    {
      key: routeName,
      title,
      children: children.length > 0 ? children : undefined,
    },
  ];
}
