<script setup lang="ts">
import { computed, ref, watch } from 'vue';

import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  message,
  Select,
  Switch,
  Tabs,
} from 'ant-design-vue';

interface SystemConfig {
  id?: string;
  config_group: string;
  config_key: string;
  name: string;
  description?: string;
  value_type: string;
  value: string;
  default_value?: string;
  is_editable: boolean;
  is_system: boolean;
  sort_order: number;
}

const props = defineProps<{
  editModel: SystemConfig;
}>();

const emit = defineEmits<{
  'update:editModel': [value: SystemConfig];
}>();

const activeTab = ref('edit');

// value_type 选项
const valueTypeOptions = [
  { label: '字符串', value: 'string' },
  { label: '整数', value: 'int' },
  { label: '布尔值', value: 'bool' },
  { label: 'JSON', value: 'json' },
  { label: '数组', value: 'array' },
];

// 本地模型，用于内部编辑
const localModel = computed({
  get: () => props.editModel,
  set: (val) => emit('update:editModel', val),
});

// 布尔值显示
const boolValue = computed({
  get: () => {
    const val = localModel.value.value;
    return val === 'true';
  },
  set: (val: boolean) => {
    localModel.value.value = val ? 'true' : 'false';
  },
});

const defaultBoolValue = computed({
  get: () => localModel.value.default_value === 'true',
  set: (val: boolean) => {
    localModel.value.default_value = val ? 'true' : 'false';
  },
});

// JSON 格式化显示（用于编辑）
const jsonValue = computed({
  get: () => {
    if (
      localModel.value.value_type === 'json' ||
      localModel.value.value_type === 'array'
    ) {
      try {
        const val = localModel.value.value;
        if (!val || val === '{}' || val === '[]') {
          return val || (localModel.value.value_type === 'array' ? '[]' : '{}');
        }
        const parsed = JSON.parse(val);
        return JSON.stringify(parsed, null, 2);
      } catch {
        return localModel.value.value;
      }
    }
    return localModel.value.value;
  },
  set: (val) => {
    localModel.value.value = val;
  },
});

// 解析后的 JSON 数据（用于预览）
const parsedJson = computed(() => {
  if (
    localModel.value.value_type !== 'json' &&
    localModel.value.value_type !== 'array'
  ) {
    return null;
  }
  try {
    const val = localModel.value.value;
    if (!val) return localModel.value.value_type === 'array' ? [] : {};
    return JSON.parse(val);
  } catch {
    return { error: '无效的 JSON 格式' };
  }
});

// JSON 占位符
const jsonPlaceholder = computed(() => {
  return localModel.value.value_type === 'array'
    ? '["item1", "item2"]'
    : '{"key": "value"}';
});

// 验证 JSON 格式
function validateJSON(value: string): boolean {
  if (!value) return true;
  try {
    JSON.parse(value);
    return true;
  } catch {
    return false;
  }
}

// JSON 验证器（用于表单规则）
function createJSONValidator() {
  return (_rule: any, value: string) => {
    if (!value) {
      return Promise.resolve();
    }
    if (validateJSON(value)) {
      return Promise.resolve();
    }
    return Promise.reject(new Error('JSON 格式错误'));
  };
}

// 格式化 JSON
function formatJSON() {
  if (
    localModel.value.value_type !== 'json' &&
    localModel.value.value_type !== 'array'
  ) {
    return;
  }
  try {
    const parsed = JSON.parse(localModel.value.value || '{}');
    localModel.value.value = JSON.stringify(parsed, null, 2);
    message.success('JSON 格式化成功');
  } catch {
    message.error('JSON 格式错误，无法格式化');
  }
}

// 压缩 JSON（单行）
function minifyJSON() {
  if (
    localModel.value.value_type !== 'json' &&
    localModel.value.value_type !== 'array'
  ) {
    return;
  }
  try {
    const parsed = JSON.parse(localModel.value.value || '{}');
    localModel.value.value = JSON.stringify(parsed);
    message.success('JSON 压缩成功');
  } catch {
    message.error('JSON 格式错误，无法压缩');
  }
}

// 提供给父组件使用的方法
defineExpose({
  getFormStateData: () => {
    // 验证 JSON 格式
    if (
      (localModel.value.value_type === 'json' ||
        localModel.value.value_type === 'array') &&
      !validateJSON(localModel.value.value)
    ) {
      throw new Error('JSON 格式错误');
    }
    return { ...localModel.value };
  },
});

// 当 value_type 改变时，提供默认值
watch(
  () => localModel.value.value_type,
  (newType, oldType) => {
    if (newType !== oldType && !localModel.value.id) {
      // 新建时设置默认值
      switch (newType) {
        case 'array': {
          localModel.value.value = '[]';
          localModel.value.default_value = '[]';
          break;
        }
        case 'bool': {
          localModel.value.value = 'false';
          localModel.value.default_value = 'false';
          break;
        }
        case 'int': {
          localModel.value.value = '0';
          localModel.value.default_value = '0';
          break;
        }
        case 'json': {
          localModel.value.value = '{}';
          localModel.value.default_value = '{}';
          break;
        }
        default: {
          localModel.value.value = '';
          localModel.value.default_value = '';
        }
      }
    }
    // 切换 tab 到编辑
    activeTab.value = 'edit';
  },
);
</script>

<template>
  <Form ref="formRef" :model="localModel" layout="vertical">
    <Form.Item
      label="配置分组"
      name="config_group"
      :rules="[{ required: true, message: '请输入配置分组' }]"
    >
      <Input
        v-model:value="localModel.config_group"
        placeholder="如: oauth, llm, app"
        :disabled="!!localModel.id"
      />
    </Form.Item>

    <Form.Item
      label="配置键"
      name="config_key"
      :rules="[{ required: true, message: '请输入配置键' }]"
    >
      <Input
        v-model:value="localModel.config_key"
        placeholder="如: api_key, timeout"
        :disabled="!!localModel.id"
      />
    </Form.Item>

    <Form.Item label="配置名称" name="name">
      <Input v-model:value="localModel.name" placeholder="显示名称" />
    </Form.Item>

    <Form.Item label="配置描述" name="description">
      <Input.TextArea
        v-model:value="localModel.description"
        :rows="2"
        placeholder="配置用途说明"
      />
    </Form.Item>

    <Form.Item
      label="值类型"
      name="value_type"
      :rules="[{ required: true, message: '请选择值类型' }]"
    >
      <Select
        v-model:value="localModel.value_type"
        :options="valueTypeOptions"
        placeholder="选择值类型"
        :disabled="!!localModel.id"
      />
    </Form.Item>

    <Form.Item
      label="配置值"
      name="value"
      :rules="[
        { required: true, message: '请输入配置值' },
        ...(localModel.value_type === 'json' ||
        localModel.value_type === 'array'
          ? [{ validator: createJSONValidator(), trigger: 'blur' as const }]
          : []),
      ]"
    >
      <!-- 布尔值 -->
      <div
        v-if="localModel.value_type === 'bool'"
        class="flex items-center gap-3"
      >
        <Switch
          v-model:checked="boolValue"
          checked-children="开启"
          un-checked-children="关闭"
        />
        <span class="text-sm text-gray-500">
          当前值: {{ localModel.value === 'true' ? 'true' : 'false' }}
        </span>
      </div>

      <!-- 整数 -->
      <InputNumber
        v-else-if="localModel.value_type === 'int'"
        v-model:value="localModel.value"
        style="width: 100%"
        :precision="0"
      />

      <!-- JSON/数组 - 使用 Tab 切换编辑/预览 -->
      <div
        v-else-if="
          localModel.value_type === 'json' || localModel.value_type === 'array'
        "
        class="json-editor"
      >
        <Tabs v-model:active-key="activeTab">
          <Tabs.TabPane key="edit" tab="编辑">
            <Input.TextArea
              v-model:value="jsonValue"
              :rows="12"
              :placeholder="jsonPlaceholder"
            />
            <div class="mt-2 flex gap-4">
              <Button type="link" size="small" @click="formatJSON">
                格式化
              </Button>
              <Button type="link" size="small" @click="minifyJSON">
                压缩
              </Button>
              <span class="text-sm text-gray-400">支持标准 JSON 格式</span>
            </div>
          </Tabs.TabPane>
          <Tabs.TabPane key="preview" tab="预览">
            <Card :bordered="true" class="json-preview-card">
              <pre v-if="parsedJson" class="json-preview">{{
                JSON.stringify(parsedJson, null, 2)
              }}</pre>
              <div v-else class="text-gray-400">无效的 JSON</div>
            </Card>
          </Tabs.TabPane>
        </Tabs>
      </div>

      <!-- 字符串 -->
      <Input.TextArea
        v-else
        v-model:value="localModel.value"
        :rows="4"
        placeholder="请输入配置值"
      />
    </Form.Item>

    <Form.Item label="默认值" name="default_value">
      <template v-if="localModel.value_type !== 'bool'">
        <Input.TextArea
          v-model:value="localModel.default_value"
          :rows="2"
          placeholder="默认值（可选）"
        />
      </template>
      <template v-else>
        <Switch v-model:checked="defaultBoolValue" />
      </template>
    </Form.Item>

    <Form.Item label="是否可编辑" name="is_editable">
      <Switch
        v-model:checked="localModel.is_editable"
        checked-children="是"
        un-checked-children="否"
      />
    </Form.Item>

    <Form.Item label="是否系统配置" name="is_system">
      <Switch
        v-model:checked="localModel.is_system"
        checked-children="是"
        un-checked-children="否"
      />
    </Form.Item>

    <Form.Item label="排序" name="sort_order">
      <InputNumber
        v-model:value="localModel.sort_order"
        style="width: 100%"
        :precision="0"
      />
    </Form.Item>
  </Form>
</template>

<style scoped>
.mt-2 {
  margin-top: 8px;
}

.flex {
  display: flex;
}

.gap-3 {
  gap: 12px;
}

.gap-4 {
  gap: 16px;
}

.text-gray-500 {
  color: #6b7280;
}

.text-gray-400 {
  color: #9ca3af;
}

.text-sm {
  font-size: 14px;
}

.json-editor {
  padding: 8px;
  background: #fafafa;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
}

.json-editor :deep(.ant-tabs-nav) {
  margin-bottom: 8px;
}

.json-preview-card {
  background: #f5f5f5;
}

.json-preview-card :deep(.ant-card-body) {
  padding: 12px;
}

.json-preview {
  max-height: 300px;
  margin: 0;
  overflow-y: auto;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 13px;
  line-height: 1.6;
  word-break: break-all;
  white-space: pre-wrap;
}
</style>
