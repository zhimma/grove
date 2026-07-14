# 系统配置敏感值设计

## 密钥边界

业务配置密文使用独立的 `CONFIG_ENCRYPTION_KEY`，不复用 JWT 密钥、数据库密码或存储密钥。密钥只来自进程配置，禁止写入 `system_configs`、API 响应、操作日志或错误详情。

密钥接受两种格式：至少 32 字节的原始字符串，或 `base64:` 前缀的 32 字节随机值。应用使用 SHA-256 规范化为 AES-256 密钥；密钥缺失时普通配置仍可用，但创建、更新或解析 secret 配置会明确失败。

## 密文格式

使用标准库 AES-256-GCM，每次加密生成独立随机 nonce。数据库格式为：

```text
v1:<base64(nonce || ciphertext || tag)>
```

版本前缀为未来轮换和算法升级保留空间。解密拒绝未知版本、非法 Base64、过短载荷和认证失败。

## 数据与 API 语义

`system_configs` 增加 `is_secret BOOLEAN NOT NULL DEFAULT FALSE`。secret 的 `value` 和 `default_value` 在校验原始值后加密入库。

列表、分组、创建和更新响应都只返回统一掩码 `********`，绝不返回密文或明文。已有 secret 更新使用显式 `keep_secret=true` 保持原值；`keep_secret=false` 时 `value` 是新的明文，空字符串表示明确清空。

内部业务读取通过 service 的解析方法完成，返回解密后的有效值；模型本身不持有密钥，也不在 JSON 序列化阶段隐式解密。

## 基础设施 secret 禁止项

业务配置表禁止存放数据库密码、Redis 密码、JWT 主密钥、配置加密密钥和对象存储长期访问密钥。创建 secret 时对保留分组和保留键做服务端拒绝，并返回稳定错误码 `infrastructure_secret_not_allowed`。

这些基础设施密钥必须来自环境变量或外部 secret manager；系统配置页面会显示相同警告。

## 审计

创建和更新系统配置的审计详情只记录：

- `config_key`
- `changed=true`
- `is_secret`

不记录 value、default_value、密文、掩码或请求体。删除操作只记录配置 ID 和删除动作。

## 前端

系统配置类型增加 `is_secret`。新建时可选择 secret；编辑时该属性不可变。已有 secret 默认开启“保持原值”，值输入框禁用；关闭后才能输入新值。列表统一显示“已设置”，不展示掩码内容。

## 验证

- secretbox 随机 nonce、往返、篡改、错误密钥和格式错误测试。
- SQLite service 测试验证数据库只存密文、API/service 输出只含掩码、保持原值、替换、清空和内部解密。
- Router 测试验证响应和操作审计不包含明文、密文或掩码字段。
- PostgreSQL 011 migration up/down 和全量迁移生命周期验证。
- Vue 类型检查和 secret 编辑交互验证。
