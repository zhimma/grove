import { consoleEndpoint } from '#/api/console-contract';
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
  public: boolean;
  region: string;
  serve_static: boolean;
  upload_mode: 'server' | 'sts';
}

export interface StoredFile {
  content_type: string;
  checksum?: string;
  disk: string;
  driver: StorageDriver;
  filename: string;
  path: string;
  public: boolean;
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
  return requestClient.get<StorageClientConfig>(
    consoleEndpoint('consoleGetStorageConfig'),
    {
      params: disk ? { disk } : undefined,
    },
  );
}

export function uploadStorageFile(input: UploadStorageFileInput) {
  return requestClient.upload<StoredFile>(
    consoleEndpoint('consoleUploadFile'),
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
