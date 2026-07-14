import { requestClient } from '#/api/request';

export type StorageDriver = 'local' | 's3';

export interface StorageClientConfig {
  base_url: string;
  bucket: string;
  disk: string;
  driver: StorageDriver;
  endpoint: string;
  is_default: boolean;
  prefix: string;
  region: string;
  upload_mode: 'server' | 'sts';
}

export interface StoredFile {
  content_type: string;
  disk: string;
  driver: StorageDriver;
  filename: string;
  path: string;
  purpose: string;
  size: number;
  url: string;
}

interface UploadStorageFileInput {
  disk?: string;
  file: File;
  onProgress?: (percentage: number) => void;
  purpose: string;
}

export function getStorageConfig(disk?: string) {
  return requestClient.get<StorageClientConfig>('/console/v1/storage/config', {
    params: disk ? { disk } : undefined,
  });
}

export function uploadStorageFile(input: UploadStorageFileInput) {
  return requestClient.upload<StoredFile>(
    '/console/v1/storage/upload',
    {
      disk: input.disk,
      file: input.file,
      purpose: input.purpose,
    },
    {
      onUploadProgress: (event) => {
        const percentage = Math.round((event.progress ?? 0) * 100);
        input.onProgress?.(percentage);
      },
    },
  );
}
