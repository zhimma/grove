import type {
  StorageAdapter,
  STSCredentials,
  UploadOptions,
  UploadResult,
} from '../adapter';

/**
 * 阿里云 OSS 存储适配器（预留实现）
 *
 * TODO: 需要安装 ali-oss 依赖
 * npm install ali-oss
 */
export class OSSAdapter implements StorageAdapter {
  private credentials: null | STSCredentials = null;

  /**
   * 生成文件访问 URL
   */
  generateUrl(name: string): string {
    if (!this.credentials) {
      throw new Error('OSS 未初始化');
    }
    const { bucket, region } = this.credentials;
    // OSS 默认访问地址格式
    return `https://${bucket}.${region}.aliyuncs.com/${name}`;
  }

  /**
   * 获取 STS 临时凭证
   */
  async getSTSCredentials(): Promise<STSCredentials> {
    // TODO: 调用后端 API 获取 OSS STS 凭证
    throw new Error('OSS 适配器尚未实现');
  }

  /**
   * 上传文件到 OSS
   */
  async upload(options: UploadOptions): Promise<UploadResult> {
    // TODO: 实现 OSS 上传逻辑
    const { file, prefix } = options;
    const fileName = this.generateFileName(file.name, prefix);
    void fileName;

    throw new Error('OSS 适配器尚未实现');
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
