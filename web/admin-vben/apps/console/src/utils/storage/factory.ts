import type { StorageAdapter, StorageConfig } from './adapter';

import { COSAdapter } from './adapters/cos-adapter';
import { OSSAdapter } from './adapters/oss-adapter';
import { S3Adapter } from './adapters/s3-adapter';

/**
 * 存储驱动类型
 */
export type StorageDriver = 'cos' | 'oss' | 's3';

/**
 * 存储工厂
 * 用于创建不同存储后端的适配器实例
 */
export const StorageFactory = {
  /**
   * 创建存储适配器实例
   * @param driver 存储驱动类型
   * @returns 存储适配器实例
   */
  create(driver: StorageDriver): StorageAdapter {
    switch (driver) {
      case 'cos': {
        return new COSAdapter();
      }
      case 'oss': {
        return new OSSAdapter();
      }
      case 's3': {
        return new S3Adapter();
      }
      default: {
        throw new Error(
          `不支持的存储驱动: ${driver}，支持的驱动: cos, oss, s3`,
        );
      }
    }
  },

  /**
   * 根据配置创建存储适配器实例
   * @param config 存储配置
   * @returns 存储适配器实例
   */
  createFromConfig(config: StorageConfig): StorageAdapter {
    return this.create(config.driver);
  },

  /**
   * 检查驱动是否受支持
   * @param driver 存储驱动类型
   * @returns 是否受支持
   */
  isSupported(driver: string): boolean {
    return ['cos', 'oss', 's3'].includes(driver);
  },
};

/**
 * 获取默认存储驱动
 * 从环境变量或配置中读取
 */
export function getDefaultStorageDriver(): StorageDriver {
  // 优先从 Vite 环境变量读取
  const driver = import.meta.env.VITE_STORAGE_DRIVER as StorageDriver;
  if (driver && StorageFactory.isSupported(driver)) {
    return driver;
  }
  // 默认使用 COS
  return 'cos';
}
