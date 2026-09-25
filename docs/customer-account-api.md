# 客户账户对外 API

本文档供下游服务对接客户兑换码、账户余额和默认 API 密钥。

## 认证

所有接口均使用客户注册、登录或 OAuth 登录返回的 `data.access_token`：

```http
Authorization: Bearer <access_token>
```

使用客户登录 JWT，不使用模型调用的 `sk-...` 密钥或管理员 API Key。所有操作只作用于 JWT 所属客户，不能通过 `user_id` 指定其他账户。登录接口继续遵守已有验证码、Turnstile 和双因素认证规则。

请求应通过 HTTPS。响应使用现有统一格式：成功时 HTTP 200、`code: 0`；失败时使用相应 HTTP 状态码，并返回 `message`，部分错误另有稳定的 `reason`。

| 功能 | 方法 | 路径 |
| --- | --- | --- |
| 兑换兑换码 | POST | `/api/v1/redeem` |
| 查询账户余额 | GET | `/api/v1/user/balance` |
| 查询最近兑换记录 | GET | `/api/v1/redeem/history` |
| 获取四个默认密钥（自动检查并补齐） | GET | `/api/v1/keys/defaults` |
| 获取单个默认密钥（自动检查并补齐） | GET | `/api/v1/keys/defaults/:purpose` |
| 补建默认密钥 | POST | `/api/v1/keys/defaults` |
| 更新某用途的默认密钥 | PUT | `/api/v1/keys/defaults/:purpose` |

默认密钥的分组、用途和完整示例见 [默认 API 密钥文档](default-api-keys.md)。更新接口接收 `{"api_key_id":205}`，将客户已有的新密钥设为默认，旧密钥保留；更新后重新 GET 默认密钥接口即可读取最新结果。

老用户首次 GET 时会自动检查四个用途：保留已有默认关联，未初始化的用途优先复用同组已有密钥，缺少时才创建。重复拉取不会重复创建；复用密钥的状态、有效期及额度限制保持不变。

## 兑换兑换码

```bash
curl -X POST 'https://YOUR_DOMAIN/api/v1/redeem' \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"code":"YOUR_REDEEM_CODE"}'
```

唯一必填字段为字符串 `code`，服务端去除首尾空白。余额、并发数、订阅类兑换码均沿用现有兑换规则；注册邀请码不能在此接口兑换。

成功返回兑换记录，示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 101,
    "code": "YOUR_REDEEM_CODE",
    "type": "balance",
    "value": 12,
    "status": "used",
    "used_by": 7,
    "used_at": "2026-09-24T03:00:00Z",
    "created_at": "2026-09-24T02:00:00Z",
    "group_id": null,
    "validity_days": 30
  }
}
```

`data.type` 为 `balance`、`concurrency` 或 `subscription`。`value` 是兑换码记录的数值，不是兑换后的余额；订阅分组与有效期见 `group_id`、`validity_days`。本接口不保证返回 `new_balance`，成功后请调用余额接口获取最新余额，勿在客户端用 `value` 自行累加展示余额。

兑换码使用状态与权益发放在数据库事务中提交。同一码只能成功兑换一次，再次提交返回 HTTP 409，不会重复加款。若网络超时导致结果不确定，应先查询兑换记录核对 `code`，再查询余额；已使用错误本身不能证明该码由当前客户兑换。兑换记录接口返回最近 25 条记录，更早的记录应由运营核查。

| HTTP 状态 | reason | 说明 |
| --- | --- | --- |
| 400 | 可能为空 | 请求格式错误、缺少兑换码或全为空白 |
| 400 | `REDEEM_CODE_UNSUPPORTED_TYPE` | 不支持该兑换码类型，例如注册邀请码 |
| 400 | `REDEEM_CODE_INVALID` | 订阅兑换码配置不完整 |
| 401 | 由认证中间件返回 | 未登录、JWT 无效或过期 |
| 404 | `REDEEM_CODE_NOT_FOUND` | 兑换码不存在 |
| 409 | `REDEEM_CODE_USED` | 兑换码已使用或不可由当前客户领取 |
| 409 | `REDEEM_CODE_EXPIRED` | 兑换码已过期 |
| 409 | `REDEEM_CODE_LOCKED` | 同一码正在处理中，可稍后核对结果或重试 |
| 429 | `REDEEM_RATE_LIMITED` | 兑换失败次数过多 |
| 5xx | 由服务端返回 | 服务异常，先核对是否已成功兑换再重试 |

## 查询账户余额

```bash
curl 'https://YOUR_DOMAIN/api/v1/user/balance' \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

无需请求体或查询参数，返回当前客户的展示余额快照：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": 7,
    "display_balance": 18
  }
}
```

| 字段 | 含义 |
| --- | --- |
| `user_id` | JWT 对应的客户 ID |
| `display_balance` | 面向客户展示的余额，与现有客户页面使用的展示余额口径一致 |

下游直接使用 `display_balance` 展示客户余额，不进行汇率转换或额外扣减。本接口不返回内部计费余额 `balance` 和内部冻结额度 `frozen_balance`。展示余额不能作为内部计费额度或调用授权依据。

该接口查询当前数据库状态，不返回密码、邮箱、默认密钥或其他资料。模型调用、充值和退款可以并发改变余额，因此结果是查询时的快照，不是额度预留或调用成功保证。接口设置 `Cache-Control: no-store`，客户端刷新余额时也应避免复用旧缓存。

## 查询兑换记录

```bash
curl 'https://YOUR_DOMAIN/api/v1/redeem/history' \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

`data` 为当前客户最近 25 条记录数组，记录格式与兑换成功返回值相同；无记录时为空数组。历史记录还可能包含管理员调整等类型。兑换和历史响应包含兑换码，客户端不要将完整响应写入公开日志。

## 部署

兑换和历史接口沿用现有路由。余额接口需要部署本次后端更新，本次余额与兑换接口接入不新增数据库迁移。它们受现有客户功能模式限制，后台禁用相关客户功能时可能返回 403。

本文档为仓库内对接文档。前台文档入口由后台 `doc_url` 配置，发布时需同步到该文档站。
