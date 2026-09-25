# 注册默认 API 密钥

账户余额查询、兑换码兑换及认证说明见 [客户账户对外 API](customer-account-api.md)。

新客户完成邮箱注册、第三方首次登录或 OAuth 注册确认后，系统自动创建以下密钥：

| purpose | 分组名称（精确匹配） |
| --- | --- |
| `text` | `OC--ChatGPT【文本模型】` |
| `image` | `OC--ChatGPT【生图】` |
| `video` | `OC--Seedance【视频】` |
| `audio` | `OC--Qwen-TTS【音频】` |

密钥名称初始与分组名称一致，状态为 active，无额外密钥额度上限和有效期；实际调用仍受客户余额、订阅、分组以及平台额度限制。正常密钥列表 `/api/v1/keys` 也会显示它们。

## 部署要求

部署新版后端并完成迁移 `192_user_default_api_keys.sql` 和 `193_user_default_api_keys_audio.sql`。四个分组必须已经存在、启用且客户有权绑定。专属分组需要授权，订阅分组需要有效订阅；注册默认赠送的订阅会先于密钥创建执行。

所有缺失密钥在同一事务中写入。分组缺失、禁用、无权限或数据库故障时，不会留下本次创建的部分密钥。客户账号仍可注册和登录，服务端记录失败原因；修复配置后，可由客户调用下方补建接口重试。

已有客户在调用 GET 或 POST `/api/v1/keys/defaults` 时自动检查四个用途并补齐，无需重新注册。此操作按当前客户触发，不会在部署时批量扫描全部客户。

已有关联的默认密钥保持不变。尚未初始化的用途，先查找当前客户在对应分组下未删除、且未被其他用途占用的密钥；优先选择与分组同名的密钥，其次选择 ID 最小的同组密钥。只有没有可复用密钥时才新建。判断以实际绑定分组为准，仅名称相同但分组不同的密钥不会被复用。已有三个用途的客户仅补齐音频用途。

复用时保留原密钥内容、名称、状态、有效期和额度限制，包括停用、过期或额度耗尽状态，不会通过新建密钥绕过原有限制。

## 客户认证及读取

使用客户注册或登录接口返回的 `data.access_token`，也支持现有 OAuth 登录获得的 access token。登录仍遵守现有验证码、Turnstile 和双因素认证规则。

```bash
curl 'https://YOUR_DOMAIN/api/v1/keys/defaults' \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

成功响应示例（`api_key` 与现有密钥详情格式一致，以下省略额度等字段）：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {"purpose": "text", "api_key": {"id": 101, "key": "sk-...", "name": "OC--ChatGPT【文本模型】", "group_id": 1, "status": "active"}},
    {"purpose": "image", "api_key": {"id": 102, "key": "sk-...", "name": "OC--ChatGPT【生图】", "group_id": 2, "status": "active"}},
    {"purpose": "video", "api_key": {"id": 103, "key": "sk-...", "name": "OC--Seedance【视频】", "group_id": 3, "status": "active"}},
    {"purpose": "audio", "api_key": {"id": 104, "key": "sk-...", "name": "OC--Qwen-TTS【音频】", "group_id": 4, "status": "active"}}
  ]
}
```

通过 `purpose` 匹配用途，取 `api_key.key` 作为模型调用的 Bearer API Key。示例中的 ID 仅为占位，真实分组 ID 由服务器查询。

GET 会先补齐尚未初始化的用途，再返回最新默认密钥；重复请求不会重复生成。已删除默认密钥仍保留用途记录，其 `api_key` 为 `null`，需要通过更新接口显式指定新密钥。停用或过期的密钥保持原状态。密钥改名不影响用途识别，后续手工修改分组时返回当前真实分组信息。

也可以按用途读取单个默认密钥：

```bash
curl 'https://YOUR_DOMAIN/api/v1/keys/defaults/video' \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

该接口同样会先补齐默认密钥，再返回对应用途的单个对象。路径 `purpose` 取值为 `text`、`image`、`video`、`audio`；例如 `video` 返回 `OC--Seedance【视频】` 分组对应的默认密钥。

## 补建与重试

```bash
curl -X POST 'https://YOUR_DOMAIN/api/v1/keys/defaults' \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

无需请求体，检查、复用及补建规则与 GET 相同。仅初始化从未创建过的用途记录；并发或重复调用不会重复生成。不会恢复已删除默认密钥，不会重新启用停用密钥，不会延长有效期。

GET 和 POST 补齐时，分组不存在或未启用会返回 HTTP 503，`reason` 为 `DEFAULT_API_KEY_GROUP_UNAVAILABLE`；无分组权限时返回 HTTP 403，`reason` 为 `DEFAULT_API_KEY_GROUP_NOT_ALLOWED`。

## 更新默认密钥

客户创建新密钥后，可以将它指定为对应用途的默认密钥。此操作只更新默认关联，保留原密钥；不会重置新旧密钥内容、停用原密钥或清空已有额度使用量。

```bash
curl -X PUT 'https://YOUR_DOMAIN/api/v1/keys/defaults/text' \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"api_key_id":205}'
```

- 路径 `purpose` 取值为 `text`、`image`、`video`、`audio`，每次更新一个用途。
- 请求体 `api_key_id` 为已有新密钥的正整数 ID，可从创建密钥响应或 `GET /api/v1/keys` 获得；不要传 `sk-...` 字符串。
- 新密钥必须属于当前客户，分组需启用且客户有绑定权限。四个默认用途都不限定为初始化时的固定分组名：`text` 可使用文本平台分组，`image` 可使用已开启生图能力的分组，`video` 可使用已支持的视频平台分组，`audio` 可使用已支持的音频平台分组。
- 新密钥必须启用、未过期，且密钥总额度未耗尽。IP 限制、速率限额、账户余额等仍按原规则生效。

成功时返回单个更新后的默认项（密钥对象其他字段省略）：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "purpose": "text",
    "api_key": {"id": 205, "key": "sk-new-key", "name": "新文本密钥", "group_id": 1, "status": "active"}
  }
}
```

相同用途反复指定同一个 ID 不会创建新密钥，也不会影响另外三个用途。未初始化的用途或已删除默认密钥留下的空用途，都允许通过此接口显式指定新的密钥。之后调用补建接口不会把关联改回旧密钥。

更新在数据库事务提交后返回成功。之后下游重新调用 `GET /api/v1/keys/defaults` 会读取最新关联和密钥内容，响应禁止缓存。该机制不是主动推送：下游若缓存了旧密钥，应在收到更新成功结果后刷新缓存，或按业务需要定期重新拉取。

| HTTP 状态 | reason | 说明 |
| --- | --- | --- |
| 400 | 可能为空 / `INVALID_API_KEY_ID` | 请求体错误或 ID 非正整数 |
| 400 | `INVALID_DEFAULT_API_KEY_PURPOSE` | 不支持的用途 |
| 400 | `DEFAULT_API_KEY_GROUP_MISMATCH` | 新密钥未绑定对应用途允许的能力分组 |
| 400 | `DEFAULT_API_KEY_UNAVAILABLE` | 新密钥停用、过期或总额度耗尽 |
| 401 | 由认证中间件返回 | JWT 缺失、无效或过期 |
| 403 | `GROUP_NOT_ALLOWED` | 客户没有绑定该分组的权限 |
| 404 | `API_KEY_NOT_FOUND` | 密钥不存在、已删除或属于其他客户 |
| 503 | `DEFAULT_API_KEY_GROUP_UNAVAILABLE` | 对应分组已停用 |

上述接口均只使用 JWT 中的客户身份，不接受指定其他客户的 ID，响应设置 `Cache-Control: no-store`。JWT 是客户管理凭据，`sk-...` 是模型调用凭据，不能混用。含密钥响应应通过 HTTPS 传输，客户端不应将完整响应写入日志。
