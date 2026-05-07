/**
 * 存储适配器接口定义
 * 所有存储后端（COS/OSS/S3）都需要实现此接口
 */

/**
 * 上传配置选项
 */
export interface UploadOptions {
  /** 要上传的文件 */
  file: File;
  /** 文件路径前缀 */
  prefix?: string;
  /** 上传进度回调 */
  onProgress?: (percentage: number) => void;
}

/**
 * 上传结果
 */
export interface UploadResult {
  /** 文件访问 URL */
  url: string;
  /** 存储的文件名（包含路径） */
  name: string;
  /** 文件大小（字节） */
  size?: number;
  /** MIME 类型 */
  mimeType?: string;
  /** ETag */
  etag?: string;
}

/**
 * STS 临时凭证
 */
export interface STSCredentials {
  /** 存储区域 */
  region: string;
  /** 存储桶名称 */
  bucket: string;
  /** 驱动类型 */
  driver: string;
  /** 临时凭证 */
  credentials: {
    expiredTime: number;
    sessionToken: string;
    tmpSecretId: string;
    tmpSecretKey: string;
  };
}

/**
 * 存储配置
 */
export interface StorageConfig {
  /** 存储驱动: cos | oss | s3 */
  driver: 'cos' | 'oss' | 's3';
  /** 存储区域 */
  region: string;
  /** 存储桶名称 */
  bucket: string;
  /** 自定义域名（可选） */
  customDomain?: string;
  /** 其他驱动特定配置 */
  [key: string]: any;
}

/**
 * 存储适配器接口
 * 所有存储后端都需要实现此接口
 */
export interface StorageAdapter {
  /**
   * 上传文件
   * @param options 上传选项
   * @returns 上传结果
   */
  upload(options: UploadOptions): Promise<UploadResult>;

  /**
   * 获取 STS 临时凭证（部分适配器可能不需要）
   * @returns STS 临时凭证
   */
  getSTSCredentials?(): Promise<STSCredentials>;

  /**
   * 生成文件 URL
   * @param name 文件名（包含路径）
   * @returns 完整访问 URL
   */
  generateUrl(name: string): string;
}
