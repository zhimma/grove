<script lang="ts" setup>
import type { UploadFile, UploadProps } from 'ant-design-vue';

import type { StorageClientConfig, StoredFile } from '#/api/core/file';

import { computed, ref, shallowRef, watch } from 'vue';

import { IconifyIcon } from '@vben/icons';

import { Button, message, Upload } from 'ant-design-vue';

import { getStorageConfig, uploadStorageFile } from '#/api/core/file';

interface Props {
  /** 当前值（文件URL） */
  value?: string | string[];
  /** 后端存储磁盘，留空时使用后端默认磁盘 */
  disk?: string;
  /** 后端命名上传策略 */
  purpose?: string;
  /** 最大文件大小（MB），仅用于前端体验，服务端策略是最终边界 */
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
  disk: undefined,
  purpose: 'avatar',
  maxSize: 5,
  accept: '.jpg,.jpeg,.png,.gif,.webp',
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

const fileList = ref<UploadFile[]>([]);
const clientConfig = shallowRef<null | StorageClientConfig>(null);

const resolveClientConfig = async () => {
  if (clientConfig.value) {
    return clientConfig.value;
  }
  const config = await getStorageConfig(props.disk);
  if (config.driver !== 'local' && config.driver !== 's3') {
    throw new Error(`后端返回了不支持的存储驱动: ${config.driver}`);
  }
  if (config.upload_mode !== 'server') {
    throw new Error('当前前端未启用 S3 STS 直传，请使用服务端上传模式');
  }
  clientConfig.value = config;
  return config;
};

const uploadProps = computed<Partial<UploadProps>>(() => ({
  accept: props.accept,
  multiple: props.multiple && props.maxCount > 1,
  disabled: props.disabled,
  showUploadList: true,
  maxCount: props.maxCount,
}));

const handleCustomRequest: NonNullable<UploadProps['customRequest']> = async (
  options,
) => {
  const file = options.file as File;

  try {
    const config = await resolveClientConfig();
    const result = await uploadStorageFile({
      file,
      disk: config.disk,
      purpose: props.purpose,
      onProgress: (percentage) => {
        options.onProgress?.({ percent: percentage });
      },
    });
    const currentFile = file as unknown as UploadFile;

    currentFile.status = 'done';
    currentFile.url = result.url;
    currentFile.thumbUrl = result.url;
    currentFile.response = result;

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
    props.onSuccess?.(result.url, file);
    emit('success', result.url, file);
    options.onSuccess?.(result as unknown as Record<string, unknown>);
  } catch (error) {
    const uploadError = error instanceof Error ? error : new Error('上传失败');
    message.error(uploadError.message);
    props.onError?.(uploadError, file);
    emit('error', uploadError, file);

    const currentFile = file as unknown as UploadFile;
    currentFile.status = 'error';
    fileList.value = fileList.value.map((item) =>
      item.uid === currentFile.uid ? { ...item, status: 'error' } : item,
    );
    options.onError?.(uploadError);
  }
};

const beforeUpload: NonNullable<UploadProps['beforeUpload']> = (file) => {
  if (props.maxSize && file.size > props.maxSize * 1024 * 1024) {
    message.error(`文件大小不能超过 ${props.maxSize}MB`);
    return false;
  }
  if (props.accept && !checkFileType(file)) {
    message.error(`只支持 ${props.accept} 格式的文件`);
    return false;
  }
  return true;
};

const checkFileType = (file: File) => {
  if (!props.accept) return true;

  const acceptTypes = props.accept.split(',').map((type) => type.trim());
  const fileType = file.type.toLowerCase();
  const fileExt = `.${file.name.split('.').pop()?.toLowerCase()}`;

  return acceptTypes.some((type) => {
    const normalized = type.toLowerCase();
    if (normalized.startsWith('.')) {
      return normalized === fileExt;
    }
    if (normalized.endsWith('/*')) {
      return fileType.startsWith(normalized.slice(0, -1));
    }
    if (normalized === '*') {
      return true;
    }
    return fileType === normalized;
  });
};

const handleChange = (info: { file: UploadFile; fileList: UploadFile[] }) => {
  fileList.value = info.fileList.map((item) => {
    const previous = fileList.value.find((current) => current.uid === item.uid);
    const response = item.response as StoredFile | undefined;
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

const handleRemove = (file: UploadFile) => {
  const index = fileList.value.findIndex((item) => item.uid === file.uid);
  if (index !== -1) {
    fileList.value.splice(index, 1);
    syncValueFromFileList();
  }
};

const syncValueFromFileList = () => {
  const urls = fileList.value
    .filter((file) => file.status === 'done' && file.url)
    .map((file) => file.url!);

  emit('update:value', props.maxCount === 1 ? urls[0] || '' : urls);
};

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

watch(
  () => props.disk,
  () => {
    clientConfig.value = null;
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
