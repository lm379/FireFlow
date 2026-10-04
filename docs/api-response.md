# 接口响应

所有 JSON 接口使用以下结构：

```json
{"code":200,"data":{"id":1},"msg":"success"}
```

- `code`：成功固定为 `200`，普通错误为对应业务状态码。
- `data`：业务数据；错误和无数据的操作返回 `null`。
- `msg`：默认成功消息为 `success`，也可返回操作的具体消息。
- `reason`：可选的机器可读原因；没有原因时不输出该字段。

HTTP 状态码保留其语义，例如创建成功仍为 HTTP `201`，其响应中的 `code` 为 `200`。普通错误使用对应的 HTTP 状态码和响应 `code`。客户端同时处理 HTTP 错误和非 `200` 的业务 `code`。

列表直接放入 `data`；地域分页返回 `data.items` 和 `data.total`。登录信息放入 `data`，包含 `token`、`user`、`is_first_login` 和 `expires_at`。执行规则及同步 IP 的提示放入 `msg`，结果字段放入 `data`。

执行规则的 `data.status` 为 `updated` 或 `unchanged`，`ip_changed` 表示云端规则是否发生变更。`previous_ip` 是更新前的云端 IP（新增规则时为空），`cloud_ip` 是操作后的云端 IP，`current_ip` 是本次获取的当前 IP。更新成功后 `cloud_ip` 与 `current_ip` 相同，不据此判定为未变动。

JWT 到期返回 HTTP `401`，响应为：

```json
{"code":40101,"data":null,"msg":"登录已超时，请重新登录","reason":"TOKEN_EXPIRED"}
```

其他认证失败的 `code` 为 `401`，原因包括 `MISSING_AUTH_HEADER`、`INVALID_AUTH_HEADER`、`TOKEN_MALFORMED`、`TOKEN_INVALID_SIGNATURE` 和 `TOKEN_INVALID`。未知 API 路径、错误请求方法及未处理异常也使用统一错误结构。

管理员初始化和重置时生成随机临时密码，只在命令行输出，数据库仅保存 bcrypt 哈希。临时密码登录后，只能访问当前用户、首次登录状态、修改密码和退出登录接口；业务接口及令牌刷新返回 HTTP `403`，`reason` 为 `PASSWORD_CHANGE_REQUIRED`。修改密码必须使用不同于旧密码的新密码；密码、令牌版本和首次登录状态在同一事务中更新，修改或重置成功后已有令牌失效。

## 启用和禁用

规则使用 `POST /api/v1/rules/:id/status`，服务器实例使用 `POST /api/v1/cloud-configs/:id/status`，均需要登录认证。

```json
{"action":"enable"}
```

禁用时传 `{"action":"disable"}`。这两个接口仅更新启用状态和更新时间，不修改规则内容、关联配置、密钥或默认配置，也不调用云端接口。重复提交相同操作会保持相同状态。

成功响应的 `data` 包含资源的 `id`，以及规则的 `enabled` 或实例的 `is_enabled`；`msg` 为操作结果。非法操作返回 `400`，资源不存在时返回 `404`。
