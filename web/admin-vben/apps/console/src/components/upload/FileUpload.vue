<script lang="ts" setup>
import type { UploadFile, UploadProps } from 'ant-design-vue';

import type {
  StorageAdapter,
  StorageConfig,
  UploadResult,
} from '#/utils/storage/adapter';
import type { StorageDriver } from '#/utils/storage/factory';

import { computed, ref, watch } from 'vue';

import { IconifyIcon } from '@vben/icons';

import { Button, message, Upload } from 'ant-design-vue';

import { StorageFactory } from '#/utils/storage/factory';

interface Props {
  /** 当前值（文件URL） */
  value?: string | string[];
  /** 存储驱动类型 */
  storageType?: StorageDriver;
  /** 存储配置（优先级高于 storageType） */
  storageConfig?: StorageConfig;
  /** 文件路径前缀 */
  prefix?: string;
  /** 最大文件大小（MB） */
  maxSize?: number;
  /** 接受的文件类型 */
  accept?: string;
  /** 是否多选 */
  multiple?: boolean;
  /** 是否禁用 */
  disabled?: boolean;
  /** 最大文件数量 */
  maxCount?: number;
  /** 列表展示类型 */
  listType?: 'picture' | 'picture-card' | 'text';
  /** 上传按钮文字 */
  uploadText?: string;
  /** 上传成功回调 */
  onSuccess?: (url: string, file: File) => void;
  /** 上传失败回调 */
  onError?: (error: Error, file: File) => void;
}

const props = withDefaults(defineProps<Props>(), {
  value: undefined,
  storageType: undefined,
  storageConfig: undefined,
  prefix: 'console/uploads',
  maxSize: 10,
  accept: undefined,
  multiple: false,
  disabled: false,
  maxCount: 1,
  listType: 'text',
  uploadText: undefined,
  onSuccess: undefined,
  onError: undefined,
});

const emit = defineEmits<{
  error: [error: Error, file: File];
  success: [url: string, file: File];
  'update:value': [value: string | string[]];
}>();

// 文件列表
const fileList = ref<UploadFile[]>([]);

// 存储适配器实例
let storageAdapter: null | StorageAdapter = null;

/**
 * 初始化存储适配器
 */
const initStorageAdapter = (): StorageAdapter => {
  if (!storageAdapter) {
    storageAdapter = props.storageConfig
      ? StorageFactory.createFromConfig(props.storageConfig)
      : StorageFactory.create(props.storageType || 'cos');
  }
  return storageAdapter;
};

/**
 * 计算上传组件属性
 */
const uploadProps = computed<Partial<UploadProps>>(() => ({
  accept: props.accept,
  multiple: props.multiple && props.maxCount > 1,
  disabled: props.disabled,
  showUploadList: true,
  maxCount: props.maxCount,
}));

/**
 * 自定义上传请求
 */
const handleCustomRequest = async (options: any) => {
  const { file, onProgress, onSuccess, onError } = options;

  try {
    const adapter = initStorageAdapter();

    const result = await adapter.upload({
      file,
      prefix: props.prefix,
      onProgress: (percentage) => {
        onProgress({ percent: percentage });
      },
    });

    const currentFile = file as UploadFile;

    if (currentFile && typeof currentFile === 'object') {
      currentFile.status = 'done';
      currentFile.url = result.url;
      currentFile.thumbUrl = result.url;
      currentFile.response = result;
    }

    fileList.value = fileList.value.map((item) =>
      item.uid === currentFile.uid
        ? {
            ...item,
            status: 'done',
            url: result.url,
            thumbUrl: result.url,
            response: result,
          }
        : item,
    );

    syncValueFromFileList();

    if (props.onSuccess) {
      props.onSuccess(result.url, file);
    }
    emit('success', result.url, file);
    onSuccess(result, currentFile);
  } catch (error) {
    const errorMsg = error instanceof Error ? error.message : '上传失败';
    message.error(errorMsg);

    if (props.onError) {
      props.onError(error as Error, file);
    }
    emit('error', error as Error, file);

    const currentFile = file as UploadFile;
    if (currentFile && typeof currentFile === 'object') {
      currentFile.status = 'error';
    }
    fileList.value = fileList.value.map((item) =>
      item.uid === currentFile.uid
        ? {
            ...item,
            status: 'error',
          }
        : item,
    );

    onError(error);
  }
};

/**
 * 上传前校验
 */
const beforeUpload = (file: File): boolean | Promise<boolean> => {
  // 文件大小校验
  if (props.maxSize && file.size > props.maxSize * 1024 * 1024) {
    message.error(`文件大小不能超过 ${props.maxSize}MB`);
    return false;
  }

  // 文件类型校验
  if (props.accept && !checkFileType(file)) {
    message.error(`只支持 ${props.accept} 格式的文件`);
    return false;
  }

  return true;
};

/**
 * 检查文件类型
 */
const checkFileType = (file: File): boolean => {
  if (!props.accept) return true;

  const acceptTypes = props.accept.split(',').map((type) => type.trim());
  const fileType = file.type;
  const fileExt = `.${file.name.split('.').pop()?.toLowerCase()}`;

  return acceptTypes.some((type) => {
    if (type.startsWith('.')) {
      return type.toLowerCase() === fileExt;
    } else if (type === 'image/*') {
      return fileType.startsWith('image/');
    } else if (type === '*') {
      return true;
    } else {
      return fileType.includes(type);
    }
  });
};

/**
 * 文件列表变化
 */
const handleChange = (info: { file: UploadFile; fileList: UploadFile[] }) => {
  fileList.value = info.fileList.map((item) => {
    const previous = fileList.value.find((current) => current.uid === item.uid);
    const response = item.response as undefined | UploadResult;
    const url = item.url || response?.url || previous?.url;

    return {
      ...previous,
      ...item,
      status: url && item.status !== 'error' ? 'done' : item.status,
      url,
      thumbUrl: item.thumbUrl || url,
    };
  });
};

/**
 * 删除文件
 */
const handleRemove = (file: UploadFile) => {
  const index = fileList.value.findIndex((f) => f.uid === file.uid);
  if (index !== -1) {
    fileList.value.splice(index, 1);
    syncValueFromFileList();
  }
};

/**
 * 更新绑定的值
 */
const syncValueFromFileList = () => {
  const urls = fileList.value
    .filter((f) => f.status === 'done' && f.url)
    .map((f) => f.url!);

  if (props.maxCount === 1) {
    emit('update:value', urls[0] || '');
  } else {
    emit('update:value', urls);
  }
};

/**
 * 监听 value 变化，同步到 fileList
 */
watch(
  () => props.value,
  (newValue) => {
    if (!newValue) {
      if (fileList.value.every((item) => item.status !== 'uploading')) {
        fileList.value = [];
      }
      return;
    }

    const urls = Array.isArray(newValue) ? newValue : [newValue];
    const normalizedUrls = urls.filter(Boolean);
    const currentDoneUrls = fileList.value
      .filter((item) => item.status === 'done' && item.url)
      .map((item) => item.url!);

    if (JSON.stringify(currentDoneUrls) === JSON.stringify(normalizedUrls)) {
      return;
    }

    const uploadingItems = fileList.value.filter(
      (item) => item.status === 'uploading',
    );
    const doneItems = normalizedUrls.map((url, index) => ({
      uid: `external-${index}`,
      name: url.split('/').pop() || `file-${index}`,
      status: 'done' as const,
      url,
      thumbUrl: url,
    }));

    fileList.value = [...uploadingItems, ...doneItems];
  },
  { immediate: true },
);

/**
 * 监听存储类型变化，重置适配器
 */
watch(
  () => props.storageType,
  () => {
    storageAdapter = null;
  },
);
</script>

<template>
  <Upload
    v-bind="uploadProps"
    v-model:file-list="fileList"
    :custom-request="handleCustomRequest"
    :before-upload="beforeUpload"
    :list-type="listType"
    @change="handleChange"
    @remove="handleRemove"
  >
    <template v-if="listType === 'picture-card'">
      <div v-if="fileList.length < maxCount">
        <IconifyIcon icon="ant-design:plus-outlined" class="text-2xl" />
        <div style="margin-top: 8px">{{ uploadText || '上传' }}</div>
      </div>
    </template>
    <template v-else>
      <Button :disabled="fileList.length >= maxCount || disabled">
        <IconifyIcon icon="ant-design:upload-outlined" class="mr-1" />
        {{ uploadText || '上传文件' }}
      </Button>
    </template>
  </Upload>
</template>
