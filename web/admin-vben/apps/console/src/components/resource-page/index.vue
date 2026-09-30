<script lang="ts" setup>
import type { FormInstance, TablePaginationConfig } from 'ant-design-vue';

import type { Component } from 'vue';

import type {
  ConsoleColumn,
  ConsoleFormField,
  ConsoleSearchField,
} from './types';

import { computed, reactive, ref, watch } from 'vue';

import {
  Button,
  Card,
  DatePicker,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Radio,
  Select,
  Space,
  Switch,
  Table,
} from 'ant-design-vue';

import FileUpload from '#/components/upload/FileUpload.vue';

import { loadFormModel, toSubmitPayload } from './form-model';

defineOptions({ name: 'ConsoleResourcePage' });

const props = defineProps<{
  // 操作列宽度，自定义操作按钮多时调大
  actionWidth?: number;
  columns: ConsoleColumn[];
  createApi?: (data: Record<string, any>) => Promise<any>;
  deleteApi?: (id: string) => Promise<any>;
  fetchApi: (params: Record<string, any>) => Promise<any>;
  // 每次加载列表都带上的固定查询参数，优先于搜索条件
  fixedParams?: Record<string, any>;
  // 由业务页面显式传入，基础组件不依赖 views 目录。
  formComponent?: Component;
  formFields?: ConsoleFormField[];
  getDetailApi?: (id: string) => Promise<any>;
  searchFields?: ConsoleSearchField[];
  statusApi?: (id: string, status: number) => Promise<any>;
  title: string;
  // 提交前改写表单数据：接收当前表单数据，返回最终提交的数据
  transformPayload?: (data: Record<string, any>) => Record<string, any>;
  updateApi?: (id: string, data: Record<string, any>) => Promise<any>;
}>();

const searchFormRef = ref<FormInstance>();
const editFormRef = ref<FormInstance>();
const componentRef = ref<{
  getFormStateData: () => Promise<Record<string, any>> | Record<string, any>;
}>();
const loading = ref(false);
const modalOpen = ref(false);
const editingId = ref('');
const dataSource = ref<any[]>([]);
const searchModel = reactive<Record<string, any>>({});
const editModel = reactive<Record<string, any>>({});
const pagination = reactive<TablePaginationConfig>({
  current: 1,
  pageSize: 10,
  total: 0,
  showQuickJumper: true,
  showSizeChanger: true,
  showTotal: (total) => `共 ${total} 条`,
});

for (const field of props.searchFields || []) {
  searchModel[field.key] = undefined;
}
for (const field of props.formFields || []) {
  editModel[field.key] = undefined;
}

const canEdit = computed(
  () =>
    !!props.updateApi && (!!props.formFields?.length || !!props.formComponent),
);
const canCreate = computed(
  () =>
    !!props.createApi && (!!props.formFields?.length || !!props.formComponent),
);

// Vue renders a bare boolean as nothing, which would leave the cell empty.
function booleanCell(record: any, column: { dataIndex?: unknown }) {
  const value =
    typeof column.dataIndex === 'string' ? record[column.dataIndex] : undefined;
  return typeof value === 'boolean' ? value : undefined;
}

async function fetchList() {
  loading.value = true;
  try {
    const res = await props.fetchApi({
      page: pagination.current,
      page_size: pagination.pageSize,
      ...searchModel,
      ...props.fixedParams,
    });
    const meta = res.meta || {};
    dataSource.value = res.list || [];
    pagination.total = meta.total || 0;
    pagination.current = meta.page || pagination.current;
    pagination.pageSize = meta.page_size || pagination.pageSize;
  } finally {
    loading.value = false;
  }
}

function handleSearch() {
  pagination.current = 1;
  fetchList();
}

function handleReset() {
  searchFormRef.value?.resetFields();
  Object.keys(searchModel).forEach((key) => {
    searchModel[key] = undefined;
  });
  pagination.current = 1;
  fetchList();
}

function handleTableChange(p: TablePaginationConfig) {
  pagination.current = p.current || 1;
  pagination.pageSize = p.pageSize || 10;
  fetchList();
}

function openCreate() {
  editingId.value = '';
  loadFormModel(editModel, {});
  modalOpen.value = true;
}

async function openEdit(record: any) {
  editingId.value = record.id;
  const source = props.getDetailApi
    ? await props.getDetailApi(record.id)
    : record;
  loadFormModel(editModel, source);
  modalOpen.value = true;
}

async function submitEdit() {
  let payload: Record<string, any>;
  if (props.formComponent) {
    if (!componentRef.value?.getFormStateData) {
      throw new Error('自定义表单尚未就绪');
    }
    payload = await componentRef.value.getFormStateData();
  } else {
    await editFormRef.value?.validate();
    payload = toSubmitPayload(props.formFields || [], editModel);
  }
  if (props.transformPayload) {
    payload = props.transformPayload(payload);
  }
  if (editingId.value && props.updateApi) {
    await props.updateApi(editingId.value, payload);
    message.success('更新成功');
  } else if (props.createApi) {
    await props.createApi(payload);
    message.success('创建成功');
  }
  modalOpen.value = false;
  fetchList();
}

function handleDelete(record: any) {
  if (!props.deleteApi) {
    return;
  }
  Modal.confirm({
    title: '确认删除',
    content: `确定删除 ${record.name || record.title || record.account || '该记录'} 吗？`,
    onOk: async () => {
      await props.deleteApi?.(record.id);
      message.success('删除成功');
      fetchList();
    },
  });
}

async function handleToggleStatus(record: any) {
  if (!props.statusApi) {
    return;
  }
  const nextStatus = record.status === 1 ? 0 : 1;
  await props.statusApi(record.id, nextStatus);
  message.success('状态已更新');
  fetchList();
}

watch(
  () => props.fetchApi,
  () => {
    fetchList();
  },
  { immediate: true },
);

// 页面自定义的行操作完成后用它刷新列表
defineExpose({ reload: fetchList });
</script>

<template>
  <Card :title="title">
    <Form ref="searchFormRef" :model="searchModel" layout="inline" class="mb-4">
      <template v-for="field in searchFields || []" :key="field.key">
        <Form.Item :label="field.label" :name="field.key">
          <Select
            v-if="field.type === 'select'"
            v-model:value="searchModel[field.key]"
            allow-clear
            style="width: 180px"
            :options="field.options"
            :placeholder="`请选择${field.label}`"
          />
          <Input
            v-else
            v-model:value="searchModel[field.key]"
            allow-clear
            :placeholder="`请输入${field.label}`"
          />
        </Form.Item>
      </template>
      <Form.Item>
        <Space>
          <Button type="primary" @click="handleSearch">查询</Button>
          <Button @click="handleReset">重置</Button>
          <Button v-if="canCreate" type="dashed" @click="openCreate">
            新增
          </Button>
        </Space>
      </Form.Item>
    </Form>

    <Table
      :columns="[
        ...columns,
        {
          title: '操作',
          key: 'action',
          width: actionWidth ?? 100,
          fixed: 'right',
        },
      ]"
      :data-source="dataSource"
      :loading="loading"
      :pagination="pagination"
      row-key="id"
      :scroll="{ x: 1200 }"
      @change="handleTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <Space>
            <slot name="actions" :record="record"></slot>
            <Button
              v-if="canEdit"
              type="link"
              size="small"
              @click="openEdit(record)"
            >
              编辑
            </Button>
            <Button
              v-if="statusApi"
              type="link"
              size="small"
              @click="handleToggleStatus(record)"
            >
              {{ record.status === 1 ? '停用' : '启用' }}
            </Button>
            <Button
              v-if="deleteApi"
              danger
              type="link"
              size="small"
              @click="handleDelete(record)"
            >
              删除
            </Button>
          </Space>
        </template>
        <slot v-else name="cell" :column="column" :record="record">
          <template v-if="booleanCell(record, column) !== undefined">
            {{ booleanCell(record, column) ? '是' : '否' }}
          </template>
        </slot>
      </template>
    </Table>

    <Modal
      v-model:open="modalOpen"
      :width="1000"
      :title="editingId ? `编辑${title}` : `新增${title}`"
      @ok="submitEdit"
    >
      <component
        :is="formComponent"
        v-if="formComponent"
        ref="componentRef"
        :edit-model="editModel"
      />
      <Form v-else ref="editFormRef" :model="editModel" layout="vertical">
        <template v-for="field in formFields || []" :key="field.key">
          <Form.Item
            :extra="field.help"
            :label="field.label"
            :name="field.key"
            :rules="
              field.required
                ? [{ required: true, message: `请输入${field.label}` }]
                : []
            "
          >
            <Input.TextArea
              v-if="field.type === 'textarea'"
              v-model:value="editModel[field.key]"
              :placeholder="field.placeholder"
              :rows="4"
            />
            <InputNumber
              v-else-if="field.type === 'number'"
              v-model:value="editModel[field.key]"
              :placeholder="field.placeholder"
              style="width: 100%"
            />
            <Select
              v-else-if="field.type === 'select'"
              v-model:value="editModel[field.key]"
              :options="field.options"
            />
            <Radio.Group
              v-else-if="field.type === 'radio'"
              v-model:value="editModel[field.key]"
            >
              <Radio
                v-for="option in field.options"
                :key="option.value"
                :value="option.value"
              >
                {{ option.label }}
              </Radio>
            </Radio.Group>
            <Switch
              v-else-if="field.type === 'switch'"
              v-model:checked="editModel[field.key]"
            />
            <DatePicker
              v-else-if="field.type === 'datetime'"
              v-model:value="editModel[field.key]"
              show-time
              value-format="YYYY-MM-DD HH:mm:ss"
              style="width: 100%"
            />
            <FileUpload
              v-else-if="field.type === 'uploadImg'"
              v-model:value="editModel[field.key]"
              :disk="field.disk"
              :purpose="field.purpose || 'avatar'"
              :accept="field.accept"
              :max-count="field.maxCount || 1"
              :max-size="field.maxSize ? field.maxSize : 5"
              list-type="picture-card"
              :upload-text="field.uploadText || '上传图片'"
            />
            <Input
              v-else
              v-model:value="editModel[field.key]"
              :placeholder="field.placeholder"
            />
          </Form.Item>
        </template>
      </Form>
    </Modal>
  </Card>
</template>
