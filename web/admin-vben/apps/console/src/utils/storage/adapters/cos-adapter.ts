import type {
  StorageAdapter,
  STSCredentials,
  UploadOptions,
  UploadResult,
} from '../adapter';

import COS from 'cos-js-sdk-v5';

import { getStorageConfig } from '#/api/core/file';

/**
 * 腾讯云 COS 存储适配器
 */
export class COSAdapter implements StorageAdapter {
  private baseUrl = '';
  private client: COS | null = null;
  private credentials: null | STSCredentials = null;

  /**
   * 生成文件访问 URL
   */
  generateUrl(name: string): string {
    if (this.baseUrl) {
      return `${this.baseUrl}/${name}`;
    }
    if (!this.credentials) {
      throw new Error('COS 未初始化，请先获取 STS 凭证');
    }
    const { bucket, region } = this.credentials;
    return `https://${bucket}.cos.${region}.myqcloud.com/${name}`;
  }

  /**
   * 获取 STS 临时凭证
   */
  async getSTSCredentials(): Promise<STSCredentials> {
    // 如果凭证未过期，直接返回缓存的凭证
    if (this.credentials) {
      const now = Math.floor(Date.now() / 1000);
      // 提前 5 分钟认为凭证过期
      if (this.credentials.credentials.expiredTime > now + 300) {
        return this.credentials;
      }
    }

    const response = await getStorageConfig('cos');
    const creds = response.credentials || {};

    const { tmpSecretId, tmpSecretKey, sessionToken = '' } = creds;
    const expiredTime = response.expiredTime ?? creds.expiredTime ?? 0;

    if (!tmpSecretId || !tmpSecretKey) {
      throw new Error('COS 临时密钥获取失败：缺少 tmpSecretId 或 tmpSecretKey');
    }

    this.baseUrl = (response.baseUrl || '').replace(/\/+$/, '');

    this.credentials = {
      region: response.region,
      bucket: response.bucket,
      driver: response.driver || 'cos',
      credentials: {
        tmpSecretId,
        tmpSecretKey,
        sessionToken: sessionToken || '',
        expiredTime: expiredTime || 0,
      },
    };

    return this.credentials;
  }

  /**
   * 上传文件到 COS
   */
  async upload(options: UploadOptions): Promise<UploadResult> {
    const client = this.initClient();
    const { file, prefix, onProgress } = options;
    const fileName = this.generateFileName(file.name, prefix);
    const credentials = await this.getSTSCredentials();

    const result = await new Promise<{ ETag: string; Location: string }>(
      (resolve, reject) => {
        client.putObject(
          {
            Bucket: credentials.bucket,
            Region: credentials.region,
            Key: fileName,
            Body: file,
            // 不设置 ACL，依赖存储桶默认权限
            ContentType: file.type || 'application/octet-stream',
            onProgress: (progressData: { percent: number }) => {
              if (onProgress) {
                onProgress(Math.round(progressData.percent || 0));
              }
            },
          },
          (err, data) => {
            if (err) {
              reject(err);
            } else {
              resolve(data as { ETag: string; Location: string });
            }
          },
        );
      },
    );

    const url = this.generateUrl(fileName);

    return {
      url,
      name: fileName,
      size: file.size,
      mimeType: file.type,
      etag: result.ETag,
    };
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

  /**
   * 初始化 COS 客户端
   */
  private initClient(): COS {
    if (!this.client) {
      this.client = new COS({
        Protocol: 'https:',
        // 每次请求都获取最新的凭证
        getAuthorization: (_, callback) => {
          this.getSTSCredentials()
            .then((credentials) => {
              callback({
                TmpSecretId: credentials.credentials.tmpSecretId,
                TmpSecretKey: credentials.credentials.tmpSecretKey,
                SecurityToken: credentials.credentials.sessionToken,
                XCosSecurityToken: credentials.credentials.sessionToken,
                ExpiredTime: credentials.credentials.expiredTime,
              } as any);
            })
            .catch((error) => {
              console.error('COS getAuthorization error:', error);
              callback({} as any);
            });
        },
      });
    }
    return this.client;
  }
}
