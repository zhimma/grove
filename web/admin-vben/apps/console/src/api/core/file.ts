import { requestClient } from '#/api/request';

export interface StorageConfigResponse {
  baseUrl: string;
  bucket: string;
  credentials: {
    expiredTime?: number;
    sessionToken?: string;
    tmpSecretId?: string;
    tmpSecretKey?: string;
  };
  disk: string;
  driver: string;
  endpoint: string;
  expiredTime: number;
  region: string;
  expiration?: string;
  requestId?: string;
  startTime?: number;
}

/**
 * 获取存储配置（支持动态获取存储驱动）
 * @param disk 存储磁盘，默认 'cos'
 */
export function getStorageConfig(disk?: string) {
  return requestClient.get<StorageConfigResponse>(
    '/console/v1/storage/config',
    {
      params: { disk: disk || 'cos' },
    },
  );
}

export function getFileList() {
  return requestClient.get<StorageConfigResponse[]>('/console/v1/files');
}
