import type { Directive, DirectiveBinding } from 'vue';

import { usePermissionStore } from '#/store';

/**
 * 权限指令 v-permission
 * 用法:
 * <a-button v-permission="'user:edit'">编辑</a-button>
 * <a-button v-permission="'user:delete'">删除</a-button>
 * <div v-permission="['user:edit', 'user:delete']">...</div>
 */
export const permissionDirective: Directive = {
  mounted(el: HTMLElement, binding: DirectiveBinding) {
    const { value } = binding;
    const permissionStore = usePermissionStore();

    if (!value) return;

    // 支持字符串或数组
    const permissions = Array.isArray(value) ? value : [value];

    // 检查是否有任意一个权限
    const hasAny = permissions.some((perm: string) =>
      permissionStore.hasPermission(perm),
    );

    // 无权限则移除元素
    if (!hasAny) {
      el.remove();
    }
  },
};

/**
 * 权限指令（与 v-permission 相反）v-permission-exclude
 * 无权限时显示，有权限时移除
 */
export const permissionExcludeDirective: Directive = {
  mounted(el: HTMLElement, binding: DirectiveBinding) {
    const { value } = binding;
    const permissionStore = usePermissionStore();

    if (!value) return;

    const permissions = Array.isArray(value) ? value : [value];

    const hasAny = permissions.some((perm: string) =>
      permissionStore.hasPermission(perm),
    );

    // 有权限则移除（与 v-permission 相反）
    if (hasAny) {
      el.remove();
    }
  },
};

/**
 * 权限指令（所有权限都必须满足）v-permission-all
 */
export const permissionAllDirective: Directive = {
  mounted(el: HTMLElement, binding: DirectiveBinding) {
    const { value } = binding;
    const permissionStore = usePermissionStore();

    if (!value) return;

    const permissions = Array.isArray(value) ? value : [value];

    // 检查是否满足所有权限
    const hasAll = permissions.every((perm: string) =>
      permissionStore.hasPermission(perm),
    );

    if (!hasAll) {
      el.remove();
    }
  },
};
