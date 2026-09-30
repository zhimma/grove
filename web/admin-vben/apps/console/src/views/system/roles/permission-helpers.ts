import type { APIPermissionTreeNode } from '#/api/permission';

export function collectLeafPermissionKeys(
  nodes: APIPermissionTreeNode[],
  targetKeys: string[],
): string[] {
  if (targetKeys.length === 0) {
    return [];
  }

  const targetSet = new Set(targetKeys);
  const result: string[] = [];

  const walk = (items: APIPermissionTreeNode[]) => {
    items.forEach((item) => {
      if (item.children && item.children.length > 0) {
        walk(item.children);
        return;
      }
      if (targetSet.has(item.key)) {
        result.push(item.key);
      }
    });
  };

  walk(nodes);
  return result;
}
