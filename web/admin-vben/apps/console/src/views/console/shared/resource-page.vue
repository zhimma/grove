<script lang="ts" setup>
import type { FormInstance, TablePaginationConfig } from 'ant-design-vue';

import type {
  ConsoleColumn,
  ConsoleFormField,
  ConsoleSearchField,
} from './types';

import { computed, defineAsyncComponent, reactive, ref, watch } from 'vue';

import {
  Button,
  Card,
  Cascader,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Select,
  Space,
  Table,
  Radio
} from 'ant-design-vue';

import FileUpload from '#/components/upload/FileUpload.vue';

defineOptions({ name: 'ConsoleResourcePage' });

const props = defineProps<{
  columns: ConsoleColumn[];
  // 自定义组件名称，需放在 src/views/custom 目录下
  componentName?: string;
  createApi?: (data: Record<string, any>) => Promise<any>;
  // 接口返回数据列表的key，默认为list
  dataKey?: string;
  // 初次加载列表时是否需要收入加入固定参数
  hasListParams?: Record<string, any>;
  deleteApi?: (id: string) => Promise<any>;
  fetchApi: (params: Record<string, any>) => Promise<any>;
  formFields?: ConsoleFormField[];
  // 是否有自定义提交函数，若有则会在默认的提交函数前调用，参数为当前表单数据，需返回一个对象作为最终提交数据
  hasCustomSubmitFun?: Function;
  // 是否有自定义搜索提交函数，若有则会在默认的搜索提交函数前调用，参数为当前搜索表单数据，需返回一个对象作为最终搜索提交数据
  hasSearchSubmitFun?: Function;
  // 是否需要编辑组织架构数据，
  isNeedEditOrganizaFun?: Function;
  // 自定义列表主键id
  customerId?: string;
  modalWidth?: number;
  searchFields?: ConsoleSearchField[];
  // 设置级联选择器数据的函数，只有当编辑或搜索需要选择组织架构时才需要传入，参数为当前编辑或搜索数据
  setCascaderData?: Function;
  statusApi?: (id: string, status: number) => Promise<any>;
  title: string;
  updateApi?: (id: string, data: Record<string, any>) => Promise<any>;
  showSendButton?: boolean;
  useDefault?: boolean;
}>();

const emit = defineEmits<{
  send: [record: any];
}>();

const searchFormRef = ref<FormInstance>();
const editFormRef = ref<FormInstance>();
const loading = ref(false);
const modalOpen = ref(false);
const editingId = ref('');
const dataSource = ref<any[]>([]);
const searchModel = reactive<Record<string, any>>({});
let editModel = reactive<Record<string, any>>({});
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

const canEdit = computed(() => !!props.updateApi && (!!props.formFields?.length || !!props.componentName));
const canCreate = computed(
  () => !!props.createApi && (!!props.formFields?.length || !!props.componentName),
);

async function fetchList() {
  loading.value = true;
  try {
    if (props.hasListParams) {
      Object.assign(searchModel, props.hasListParams);
    }
    let payload = { ...searchModel };
    if (props.hasSearchSubmitFun) {
      payload = props.hasSearchSubmitFun(payload);
    }
    const res = await props.fetchApi({
      page: pagination.current,
      page_size: pagination.pageSize,
      ...payload,
    });
    const meta = res.meta || {};
    dataSource.value = props.dataKey
      ? res[props.dataKey] || []
      : res.list || [];
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
  Object.keys(editModel).forEach((key) => {
    editModel[key] = undefined;
  });
  modalOpen.value = true;
}

function openEdit(record: any) {
  // 如果有customerId，则说明需要使用record[customerId]作为编辑接口的id参数，否则使用record.id
  editingId.value = props.customerId ? record[props.customerId] : record.id;
  if (props.isNeedEditOrganizaFun) {
    const organizedData = props.isNeedEditOrganizaFun(record);
    Object.assign(editModel, organizedData);
    props.setCascaderData ? props.setCascaderData(editModel) : null;
  } else {
    Object.keys(record).forEach((key) => {
      editModel[key] = record[key];
    });
  }
  modalOpen.value = true;
}

async function submitEdit() {
  if (props.componentName && componentRef.value?.getFormStateData) {
    // 自定义组件获取数据
    let payload = componentRef.value.getFormStateData();
    if (props.hasCustomSubmitFun) {
      payload = props.hasCustomSubmitFun(payload);
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
    return;
  }
  
  await editFormRef.value?.validate();
  let payload = { ...editModel };

  if (props.hasCustomSubmitFun) {
    payload = props.hasCustomSubmitFun(payload);
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
    content: `确定删除 ${record.name ||
      record.title ||
      record.order_no ||
      record.aftersale_no ||
      record.statement_no ||
      record.account ||
      '该记录'
      } 吗？`,
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

// 状态
const loadedCustomComponent = ref<any>(null);
const componentRef = ref<any>(null);
const modalLoading = ref(false);
const error = ref<Error | null>(null);

// 异步加载自定义组件
const loadCustomComponent = async (componentName?: string) => {
  if (!componentName) {
    loadedCustomComponent.value = null;
    return;
  }

  modalLoading.value = true;
  error.value = null;

  try {
    const componentModule = await import(
      /* webpackChunkName: "custom-[request]" */
      `../custom/${componentName}.vue`
    );

    loadedCustomComponent.value = defineAsyncComponent(() =>
      Promise.resolve(componentModule.default || componentModule),
    );
  } catch (error_) {
    console.error(`加载组件 ${componentName} 失败:`, error_);
    error.value = error_ instanceof Error ? error_ : new Error(String(error_));
    loadedCustomComponent.value = null;
  } finally {
    modalLoading.value = false;
  }
};

// 监听组件名称变化
watch(
  () => props.componentName,
  (newName, oldName) => {
    if (newName !== oldName && newName && !props.useDefault) {
      loadCustomComponent(newName);
    }
  },
  { immediate: true },
);

watch(
  () => props.fetchApi,
  () => {
    fetchList();
  },
  { immediate: true },
);
</script>

<template>
  <Card :title="title">
    <Form ref="searchFormRef" :model="searchModel" layout="inline" class="mb-4">
      <template v-for="field in searchFields || []" :key="field.key">
        <Form.Item :label="field.label" :name="field.key">
          <Input v-if="!field.type || field.type === 'input'" v-model:value="searchModel[field.key]" allow-clear
            :placeholder="`请输入${field.label}`" />
          <Select v-else-if="field.type === 'select'" v-model:value="searchModel[field.key]" allow-clear
            style="width: 180px" :options="field.options" :placeholder="`请选择${field.label}`" @change="
              field.haschange ? field.onChange?.(searchModel[field.key]) : null
              " />
          <Cascader v-else-if="field.type === 'cascader'" style="width: 180px" v-model:value="searchModel[field.key]"
            :options="field.options" :load-data="field.loadData" :placeholder="`请选择${field.label}`" change-on-select />
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

    <Table :columns="[
      ...columns,
      { title: '操作', key: 'action', width: 100, fixed: 'right' },
    ]" :data-source="dataSource" :loading="loading" :pagination="pagination" row-key="id" :scroll="{ x: 1200 }"
      @change="handleTableChange">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <Space>
            <Button v-if="canEdit" type="link" size="small" @click="openEdit(record)">
              编辑
            </Button>
            <Button v-if="statusApi" type="link" size="small" @click="handleToggleStatus(record)">
              {{ record.status === 1 ? '停用' : '启用' }}
            </Button>
            <Button v-if="deleteApi" danger type="link" size="small" @click="handleDelete(record)">
              删除
            </Button>
            <Button v-if="showSendButton" type="link" size="small" @click="emit('send', record)">
              发送
            </Button>
          </Space>
        </template>
      </template>
    </Table>

    <Modal v-model:open="modalOpen" :width="props.modalWidth || 1000" :title="editingId ? `编辑${title}` : `新增${title}`"
      @ok="submitEdit">
      <component v-if="props.componentName" :is="loadedCustomComponent" :editModel="editModel" ref="componentRef" />
      <Form v-else ref="editFormRef" :model="editModel" layout="vertical">
        <template v-for="field in formFields || []" :key="field.key">
          <Form.Item v-if="!field.isNotShow" :label="field.label" :name="field.key" :rules="field.required
            ? [{ required: true, message: `请输入${field.label}` }]
            : []
            ">
            <Input v-if="!field.type || field.type === 'input'" v-model:value="editModel[field.key]" />
            <Input.TextArea v-else-if="field.type === 'textarea'" v-model:value="editModel[field.key]" :rows="4" />
            <InputNumber v-else-if="field.type === 'number'" v-model:value="editModel[field.key]" style="width: 100%" />
            <Select v-else-if="field.type === 'select'" v-model:value="editModel[field.key]" :options="field.options"
              @change="
                field.haschange ? field.onChange?.(editModel[field.key]) : null
                " />

            <Radio.Group v-else-if="field.type === 'radio'" v-model:value="editModel[field.key]">
              <Radio v-for="option in field.options" :key="option.value" :value="option.value">
                {{ option.label }}
              </Radio>
            </Radio.Group>

            <Cascader v-else-if="field.type === 'cascader'" v-model:value="editModel[field.key]"
              :options="field.options" :load-data="field.loadData" placeholder="请选择" change-on-select />

            <!-- <OssUpload v-else-if="field.type === 'uploadImg'" :sts-endpoint="field.stsEndpoint" :prefix="field.prefix"
              :max-size="field.maxSize ? field.maxSize : 50" :accept="field.accept || '.jpg,.jpeg,.png,.pdf'"
              @success="handleSuccess" @error="handleError" /> -->

            <FileUpload v-else-if="field.type === 'uploadImg'" v-model:value="editModel[field.key]"
              :storage-type="field.storageType || 'cos'" :prefix="field.prefix" :max-count="field.maxCount || 1"
              :max-size="field.maxSize ? field.maxSize : 50" list-type="picture-card"
              :upload-text="field.uploadText || '上传图片'" />
          </Form.Item>
        </template>
      </Form>
    </Modal>
  </Card>
</template>
