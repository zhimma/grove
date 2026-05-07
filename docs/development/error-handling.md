# 错误处理

本文档说明 Grove 中业务错误、参数错误和系统错误的处理方式。

## 核心原则

- handler 统一通过 `response.Fail` 输出错误响应
- service 返回业务错误，不直接拼 HTTP 响应
- 参数错误、权限错误、资源不存在和系统错误应明确区分

## 最短路径

### Service 返回业务错误

```go
return nil, pkgerrors.Conflict().WithMessage("角色编码已存在")
```

### Handler 输出错误响应

```go
if err != nil {
	response.Fail(c, err)
	return
}
```

## 常用错误类型

- `InvalidParams`
- `Unauthorized`
- `Forbidden`
- `NotFound`
- `Conflict`
- `TooManyRequests`
- `Internal`

## 使用约定

- 参数校验错误优先使用 `validation` 层返回
- 业务可预期错误返回明确的 `pkg/errors` 类型
- 底层异常应通过 `WithCause(err)` 保留原始错误
- 日志记录放在统一日志链路，不在每个返回点重复打印

## 边界

- service 不直接决定 JSON 结构
- panic 由 recovery 中间件统一接管
- 客户端展示文案依赖 `message`，流程控制依赖 HTTP 状态码

## 相关文档

- [响应与错误处理规范](../04-%E5%93%8D%E5%BA%94%E4%B8%8E%E9%94%99%E8%AF%AF%E5%A4%84%E7%90%86%E8%A7%84%E8%8C%83.md)
- [服务层](../guide/service.md)
