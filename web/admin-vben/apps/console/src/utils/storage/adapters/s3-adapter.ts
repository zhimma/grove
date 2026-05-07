import type {
  StorageAdapter,
  STSCredentials,
  UploadOptions,
  UploadResult,
} from '../adapter';

/**
 * AWS S3 存储适配器（预留实现）
 *
 * TODO: 需要安装 @aws-sdk/client-s3 依赖
 * npm install @aws-sdk/client-s3 @aws-sdk/lib-storage
 */
export class S3Adapter implements StorageAdapter {
  private credentials: null | STSCredentials = null;

  /**
   * 生成文件访问 URL
   */
  generateUrl(name: string): string {
    if (!this.credentials) {
      throw new Error('S3 未初始化');
    }
    const { bucket, region } = this.credentials;
    // S3 默认访问地址格式
    return `https://${bucket}.s3.${region}.amazonaws.com/${name}`;
  }

  /**
   * 获取 STS 临时凭证
   */
  async getSTSCredentials(): Promise<STSCredentials> {
    // TODO: 调用后端 API 获取 S3 临时凭证
    throw new Error('S3 适配器尚未实现');
  }

  /**
   * 上传文件到 S3
   */
  async upload(options: UploadOptions): Promise<UploadResult> {
    // TODO: 实现 S3 上传逻辑（可使用 multipart upload 支持大文件）
    const { file, prefix } = options;
    const fileName = this.generateFileName(file.name, prefix);
    void fileName;

    throw new Error('S3 适配器尚未实现');
  }

  /**
   * 生成文件名
   */
  private generateFileName(originalName: string, prefix?: string): string {
    const timestamp = Date.now();
    const random = Math.random().toString(36).slice(2, 10);
    const ext = originalName.slice(Math.max(0, originalName.lastIndexOf('.')));
    const cleanPrefix = prefix ? prefix.replaceAll(/^\/+|\/+$/g, '') : '';

    return cleanPrefix
      ? `${cleanPrefix}/${timestamp}_${random}${ext}`
      : `${timestamp}_${random}${ext}`;
  }
}
