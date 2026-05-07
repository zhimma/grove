/**
 * 存储适配器模块入口
 * 提供统一的上传接口，支持 COS、OSS、S3 等多种存储后端
 */

// 导出接口和类型
export type {
  StorageAdapter,
  StorageConfig,
  STSCredentials,
  UploadOptions,
  UploadResult,
} from './adapter';

// 导出适配器（供高级使用）
export { COSAdapter } from './adapters/cos-adapter';
export { OSSAdapter } from './adapters/oss-adapter';

export { S3Adapter } from './adapters/s3-adapter';
// 导出工厂类
export { getDefaultStorageDriver, StorageFactory } from './factory';
export type { StorageDriver } from './factory';
