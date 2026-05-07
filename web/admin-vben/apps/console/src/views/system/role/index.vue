<script setup lang="ts">
import type {
  FormInstance,
  TablePaginationConfig,
  TreeProps,
} from 'ant-design-vue';

import type { APIPermissionTreeNode } from '#/api/core/permission';
import type { CreateRoleParams, Role, UpdateRoleParams } from '#/api/core/role';

import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import { useVbenModal } from '@vben/common-ui';

import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tabs,
  Tag,
  Tree,
} from 'ant-design-vue';

import { getApiPermissionOptions } from '#/api/core/permission';
import {
  assignRoleMenus,
  assignRolePermissions,
  createRole,
  deleteRole,
  getRoleList,
  getRoleMenus,
  getRolePermissions,
  updateRole,
} from '#/api/core/role';
import { buildConsoleMenuPermissionTree } from '#/router/menu-access';
import { accessRoutes } from '#/router/routes';
import { usePermissionStore } from '#/store';
import { parseApiError } from '#/utils/http-error';

defineOptions({ name: 'ConsoleRoles' });

interface PermissionNode {
  children?: PermissionNode[];
  key: string;
  title: string;
}

function collectLeafPermissionKeys(
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

const permissionStore = usePermissionStore();

const canCreate = computed(() =>
  permissionStore.hasApiPermission('POST', '/console/v1/roles'),
);
const canUpdate = computed(() =>
  permissionStore.hasApiPermission('PUT', '/console/v1/roles/:id'),
);
const canDelete = computed(() =>
  permissionStore.hasApiPermission('DELETE', '/console/v1/roles/:id'),
);
const canAssignApiPermission = computed(() =>
  permissionStore.hasApiPermission('POST', '/console/v1/roles/:id/permissions'),
);
const canAssignMenuPermission = computed(() =>
  permissionStore.hasApiPermission('POST', '/console/v1/roles/:id/menus'),
);
const canAssignPermission = computed(
  () => canAssignApiPermission.value || canAssignMenuPermission.value,
);

const searchFormRef = ref<FormInstance>();
const searchForm = reactive({
  keyword: '',
  status: undefined as number | undefined,
});

const loading = ref(false);
const dataSource = ref<Role[]>([]);
const pagination = reactive<TablePaginationConfig>({
  current: 1,
  pageSize: 10,
  total: 0,
  showQuickJumper: true,
  showSizeChanger: true,
  showTotal: (total) => `共 ${total} 条`,
});

const columns = [
  { title: '角色名称', dataIndex: 'name', key: 'name', width: 150 },
  { title: '角色编码', dataIndex: 'code', key: 'code', width: 150 },
  {
    title: '显示名称',
    dataIndex: 'display_name',
    key: 'display_name',
    width: 150,
  },
  {
    title: '描述',
    dataIndex: 'description',
    key: 'description',
    ellipsis: true,
  },
  { title: '排序', dataIndex: 'sort', key: 'sort', width: 80 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: '操作', key: 'action', width: 280, fixed: 'right' as const },
];

async function fetchRoleList() {
  loading.value = true;
  try {
    const res = await getRoleList({
      page: pagination.current || 1,
      page_size: pagination.pageSize || 10,
      keyword: searchForm.keyword || undefined,
      status: searchForm.status,
    });
    dataSource.value = res.list;
    pagination.total = res.meta.total;
    pagination.current = res.meta.page;
    pagination.pageSize = res.meta.page_size;
  } finally {
    loading.value = false;
  }
}

function handleSearch() {
  pagination.current = 1;
  fetchRoleList();
}

function handleReset() {
  searchFormRef.value?.resetFields();
  pagination.current = 1;
  fetchRoleList();
}

function handleTableChange(p: TablePaginationConfig) {
  pagination.current = p.current;
  pagination.pageSize = p.pageSize;
  fetchRoleList();
}

function handleDelete(record: Role) {
  if (record.is_super) {
    message.warning('超级管理员角色不可删除');
    return;
  }
  Modal.confirm({
    title: '确认删除',
    content: `确定要删除角色 "${record.display_name || record.name}" 吗？`,
    onOk: async () => {
      await deleteRole(record.id);
      message.success('删除成功');
      fetchRoleList();
    },
  });
}

const [RoleModal, roleModalApi] = useVbenModal({
  onConfirm: async () => {
    try {
      await roleFormRef.value?.validate();
    } catch {
      return;
    }

    syncRoleFieldErrors({});

    try {
      if (isEdit.value) {
        await updateRole(currentRoleId.value, roleForm);
        message.success('更新成功');
      } else {
        await createRole(roleForm as CreateRoleParams);
        message.success('创建成功');
      }
    } catch (error) {
      const parsed = parseApiError(error);
      if (Object.keys(parsed.fieldErrors).length > 0) {
        syncRoleFieldErrors(parsed.fieldErrors);
        return;
      }
      throw error;
    }

    roleModalApi.close();
    fetchRoleList();
  },
});

const roleFormRef = ref<FormInstance>();
const isEdit = ref(false);
const currentRoleId = ref('');
const roleForm = reactive<CreateRoleParams & UpdateRoleParams>({
  name: '',
  code: '',
  display_name: '',
  description: '',
  sort: 0,
  status: 1,
});

const roleFieldNames = ['name', 'code', 'display_name', 'description'] as const;
const roleFieldErrors = reactive<Record<(typeof roleFieldNames)[number], string[]>>({
  name: [],
  code: [],
  display_name: [],
  description: [],
});

function syncRoleFieldErrors(fieldErrors: Record<string, string[]>) {
  roleFieldNames.forEach((name) => {
    roleFieldErrors[name] = fieldErrors[name] || [];
  });
}

function getRoleFieldError(name: (typeof roleFieldNames)[number]) {
  return roleFieldErrors[name][0] || '';
}

function getRoleFieldStatus(name: (typeof roleFieldNames)[number]) {
  return roleFieldErrors[name].length > 0 ? 'error' : undefined;
}

function handleAdd() {
  isEdit.value = false;
  currentRoleId.value = '';
  Object.assign(roleForm, {
    name: '',
    code: '',
    display_name: '',
    description: '',
    sort: 0,
    status: 1,
  });
  roleModalApi.setState({ title: '新增角色' }).open();
  nextTick(() => syncRoleFieldErrors({}));
}

function handleEdit(record: Role) {
  if (record.name === 'root') {
    message.warning('Root 角色不可编辑');
    return;
  }
  isEdit.value = true;
  currentRoleId.value = record.id;
  Object.assign(roleForm, {
    name: record.name,
    code: record.code,
    display_name: record.display_name,
    description: record.description,
    sort: record.sort,
    status: record.status,
  });
  roleModalApi.setState({ title: '编辑角色' }).open();
  nextTick(() => syncRoleFieldErrors({}));
}

const [PermissionModal, permissionModalApi] = useVbenModal({
  onConfirm: async () => {
    const tasks: Promise<unknown>[] = [];
    if (canAssignApiPermission.value) {
      const apiPermissionKeys = collectLeafPermissionKeys(
        permissionTreeData.value,
        checkedPermissionKeys.value,
      );
      tasks.push(
        assignRolePermissions(currentRoleId.value, {
          api_permissions: apiPermissionKeys,
        }),
      );
    }
    if (canAssignMenuPermission.value) {
      tasks.push(
        assignRoleMenus(currentRoleId.value, {
          menu_keys: checkedMenuKeys.value,
        }),
      );
    }
    await Promise.all(tasks);
    message.success('授权保存成功');
    permissionModalApi.close();
  },
});

const permissionLoading = ref(false);
const permissionTreeData = ref<APIPermissionTreeNode[]>([]);
const menuTreeData = ref<PermissionNode[]>([]);
const checkedPermissionKeys = ref<string[]>([]);
const checkedMenuKeys = ref<string[]>([]);

async function loadCatalogs() {
  permissionTreeData.value = await getApiPermissionOptions();
  menuTreeData.value = buildConsoleMenuPermissionTree(accessRoutes);
}

async function handleAssignPermission(record: Role) {
  if (record.name === 'root') {
    message.warning('Root 角色拥有所有权限，无需配置');
    return;
  }

  currentRoleId.value = record.id;
  permissionModalApi
    .setState({
      title: `授权配置 - ${record.display_name || record.name}`,
    })
    .open();
  permissionLoading.value = true;

  try {
    if (
      permissionTreeData.value.length === 0 ||
      menuTreeData.value.length === 0
    ) {
      await loadCatalogs();
    }
    const [permissions, menus] = await Promise.all([
      getRolePermissions(record.id),
      getRoleMenus(record.id),
    ]);
    checkedPermissionKeys.value = permissions;
    checkedMenuKeys.value = menus;
  } finally {
    permissionLoading.value = false;
  }
}

const onPermissionCheck: TreeProps['onCheck'] = (checkedKeysValue) => {
  checkedPermissionKeys.value = checkedKeysValue as string[];
};

const onMenuCheck: TreeProps['onCheck'] = (checkedKeysValue) => {
  checkedMenuKeys.value = checkedKeysValue as string[];
};

onMounted(async () => {
  fetchRoleList();
  await loadCatalogs();
});
</script>

<template>
  <div class="p-4">
    <Card title="角色管理" :bordered="false">
      <Form
        ref="searchFormRef"
        :model="searchForm"
        layout="inline"
        class="mb-4"
        @finish="handleSearch"
      >
        <Form.Item label="关键词" name="keyword">
          <Input
            v-model:value="searchForm.keyword"
            placeholder="角色名称/显示名称"
            allow-clear
            style="width: 200px"
          />
        </Form.Item>
        <Form.Item label="状态" name="status">
          <Select
            v-model:value="searchForm.status"
            placeholder="全部状态"
            allow-clear
            style="width: 120px"
            :options="[
              { label: '启用', value: 1 },
              { label: '禁用', value: 0 },
            ]"
          />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" html-type="submit">搜索</Button>
            <Button @click="handleReset">重置</Button>
          </Space>
        </Form.Item>
      </Form>

      <div class="mb-4">
        <Space>
          <Button v-if="canCreate" type="primary" @click="handleAdd">
            新增角色
          </Button>
        </Space>
      </div>

      <Table
        :columns="columns"
        :data-source="dataSource"
        :loading="loading"
        :pagination="pagination"
        row-key="id"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <Tag :color="(record as Role).status === 1 ? 'success' : 'default'">
              {{ (record as Role).status === 1 ? '启用' : '禁用' }}
            </Tag>
          </template>
          <template v-if="column.key === 'action'">
            <Space>
              <Button
                v-if="canAssignPermission"
                type="link"
                size="small"
                @click="handleAssignPermission(record as Role)"
              >
                授权
              </Button>
              <Button
                v-if="canUpdate && (record as Role).name !== 'root'"
                type="link"
                size="small"
                @click="handleEdit(record as Role)"
              >
                编辑
              </Button>
              <Button
                v-if="canDelete && !(record as Role).is_super"
                type="link"
                danger
                size="small"
                @click="handleDelete(record as Role)"
              >
                删除
              </Button>
            </Space>
          </template>
        </template>
      </Table>
    </Card>

    <RoleModal>
      <Form
        ref="roleFormRef"
        :model="roleForm"
        layout="vertical"
        :rules="{
          name: [
            { required: true, message: '请输入角色名称', trigger: 'blur' },
          ],
          code: [
            { required: !isEdit, message: '请输入角色编码', trigger: 'blur' },
          ],
        }"
      >
        <Form.Item
          label="角色名称"
          name="name"
          :help="getRoleFieldError('name')"
          :validate-status="getRoleFieldStatus('name')"
        >
          <Input v-model:value="roleForm.name" placeholder="如：运营专员" />
        </Form.Item>
        <Form.Item
          v-if="!isEdit"
          label="角色编码"
          name="code"
          extra="角色唯一标识，创建后不可修改"
          :help="getRoleFieldError('code')"
          :validate-status="getRoleFieldStatus('code')"
        >
          <Input v-model:value="roleForm.code" placeholder="如：operator" />
        </Form.Item>
        <Form.Item
          label="显示名称"
          name="display_name"
          :help="getRoleFieldError('display_name')"
          :validate-status="getRoleFieldStatus('display_name')"
        >
          <Input v-model:value="roleForm.display_name" placeholder="如：编辑" />
        </Form.Item>
        <Form.Item
          label="描述"
          name="description"
          :help="getRoleFieldError('description')"
          :validate-status="getRoleFieldStatus('description')"
        >
          <Input.TextArea
            v-model:value="roleForm.description"
            :rows="3"
            placeholder="角色描述"
          />
        </Form.Item>
        <Form.Item label="排序" name="sort">
          <InputNumber
            v-model:value="roleForm.sort"
            :min="0"
            style="width: 100%"
          />
        </Form.Item>
        <Form.Item v-if="isEdit" label="状态" name="status">
          <Switch
            v-model:checked="roleForm.status"
            :checked-value="1"
            :un-checked-value="0"
            checked-children="启用"
            un-checked-children="禁用"
          />
        </Form.Item>
      </Form>
    </RoleModal>

    <PermissionModal :width="860">
      <Spin :spinning="permissionLoading">
        <Tabs>
          <Tabs.TabPane key="api" tab="接口权限">
            <Tree
              v-model:checked-keys="checkedPermissionKeys"
              :tree-data="permissionTreeData"
              checkable
              :selectable="false"
              :default-expand-all="true"
              @check="onPermissionCheck"
            />
          </Tabs.TabPane>
          <Tabs.TabPane key="menu" tab="菜单权限">
            <Tree
              v-model:checked-keys="checkedMenuKeys"
              :tree-data="menuTreeData"
              checkable
              :selectable="false"
              :default-expand-all="true"
              @check="onMenuCheck"
            />
          </Tabs.TabPane>
        </Tabs>
      </Spin>
    </PermissionModal>
  </div>
</template>
