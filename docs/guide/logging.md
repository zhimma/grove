# Console 日志与审计

本文说明 Console 操作日志、登录日志的当前接口契约，以及审计数据的隐私和保留边界。

## 接口契约

日志接口统一使用标准成功响应。响应 `data` 是 `list + meta`：

```json
{
  "list": [],
  "meta": {
    "total": 0,
    "page": 1,
    "page_size": 20,
    "total_pages": 0
  }
}
```

当前路由（前缀为 `/console/v1`）只有：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/logs/operations` | 操作日志列表 |
| `GET` | `/logs/operations/{id}` | 操作日志详情（含 `log` 和 `detail`） |
| `GET` | `/logs/logins` | 登录日志列表 |

列表筛选使用后端字段名：

- `success=true/false`：成功或失败；不使用 `status=1/2` 映射。
- `created_from`、`created_to`：创建时间范围。
- `page`、`page_size`、`offset`、`limit`、`list_all`、`keyword`、`order_by`：通用列表参数。
- 操作日志另有 `method`、`module`；两类日志都支持 `admin_id`。

日志是审计证据，当前没有删除单条记录或清空记录的 Console API。前端页面因此只提供查询和详情，不调用不存在的 `delete` 或 `clear` 路由。

## 脱敏和长度上限

审计中保存的是排障所需的最小元数据，不是原始请求归档：

- 查询参数先解析再脱敏，敏感键（如 `authorization`、`cookie`、`password`、`token`、`secret`、`api_key`、`signature`、`code` 等）值统一写成 `REDACTED`，最终长度最多 2,000 字节。
- `system-configs` 路由不保留查询参数；这类请求可能包含配置键和值。
- `detail`（用于业务审计的请求体与请求头元数据）递归按键脱敏；字符串单值最多 2,000 字节，序列化后的 `detail_json` 最多 8,000 字节。
- User-Agent 最多 500 字节；数据库字段的路径、动作、请求 ID、目标类型和目标 ID 也按模型上限截断。
- 不读取或持久化原始 `Authorization` 请求头和原始请求体。若业务把必要的请求头与请求体元数据放入 `AuditMeta.Detail`，必须使用结构化键，中间件会再次递归脱敏。

脱敏不能替代访问控制：日志列表和详情仍需 Console 身份及 API 权限，生产环境不应把审计表暴露给匿名用户。

## 保留与清理

应用层不自动删除审计记录，也不提供绕过审批的清理按钮；这避免管理员误删证据。当前 `config.yaml` 没有审计记录保留天数配置项，默认由数据库保留策略决定（未配置时持续保留）。运行日志文件的轮转与清理另见[日志配置](configuration.md#log)。

生产环境应由数据或运维负责人根据合规要求单独制定保留期和清理作业，并在执行前完成备份、审批和影响评估。清理作业只删除超过批准期限的 `created_at` 记录，保留作业日志和执行审计；修改保留期不应通过前端页面或未审查的 SQL 临时操作完成。

新增自动清理任务或配置键时，必须同时更新本文、OpenAPI 契约和回归测试，并明确时区、失败重试及多实例幂等语义。
