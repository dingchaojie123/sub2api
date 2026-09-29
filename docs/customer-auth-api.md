# 客户注册与登录对外 API 接入文档

适用范围：下游网站、应用或服务接入本平台的邮箱注册、密码登录及登录凭证管理。本文按 2026-09-28 仓库实现整理，不包含第三方 OAuth 登录。

本文中的域名、邮箱、密码、验证码和 Token 都是示例值。第 8 节提供无需连接真实平台的可执行 HTTP 模拟测试；真实环境必须使用收到的邮箱验证码和实际签发的 Token。

## 1. 接入约定

- 站点地址示例：`https://YOUR_DOMAIN`。以下路径已经包含 `/api/v1`，不要重复拼接。
- POST 请求使用 `Content-Type: application/json`，正式接入使用 HTTPS。
- 注册、发送验证码、登录、2FA、刷新和退出接口不要求预先登录，也不使用管理员密钥或模型调用的 `sk-...` 密钥。
- 登录后的账户接口使用 `Authorization: Bearer <access_token>`，身份由 JWT 确定，不通过 `user_id` 指定客户。
- 浏览器跨域直连接口时，需要在平台 CORS 配置中允许下游网站来源。若通过下游服务转发，注意认证限流按平台识别的客户端 IP 计数，共享出口可能共用额度。
- 若启用会话绑定，登录和刷新时平台看到的 IP、User-Agent 应保持一致，否则可能被撤销会话。保持调用链路一致，客户端不能自行伪造可信代理信息。

成功响应为 HTTP 200：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

业务错误响应示例，HTTP 400：

```json
{
  "code": 400,
  "message": "email verification is required",
  "reason": "EMAIL_VERIFY_REQUIRED"
}
```

客户端先判断 HTTP 状态，再读取 `code`、`reason` 和 `message`。`reason` 不保证存在；网关错误也可能不是 JSON。路由限流的 429 格式另见第 7 节。

## 2. 接口与调用顺序

| 功能 | 方法 | 路径 | 是否需要登录 JWT |
| --- | --- | --- | --- |
| 获取公开配置 | GET | `/api/v1/settings/public` | 否 |
| 发送注册邮箱验证码 | POST | `/api/v1/auth/send-verify-code` | 否 |
| 注册 | POST | `/api/v1/auth/register` | 否 |
| 密码登录 | POST | `/api/v1/auth/login` | 否 |
| 完成登录 2FA | POST | `/api/v1/auth/login/2fa` | 否，使用临时令牌 |
| 获取当前用户 | GET | `/api/v1/auth/me` | 是 |
| 刷新登录凭证 | POST | `/api/v1/auth/refresh` | 否，使用刷新令牌 |
| 退出登录 | POST | `/api/v1/auth/logout` | 否，提交要撤销的刷新令牌 |

开启邮箱验证后的注册流程：

```text
读取公开配置
  -> 用户填写邮箱
  -> 完成 Turnstile（若启用）
  -> 发送验证码
  -> 用户从邮件读取验证码
  -> 提交邮箱、密码、验证码注册
  -> 获取 access_token，注册成功即登录
  -> 使用 JWT 查询当前用户、余额和默认 API 密钥
```

已有账户登录流程：

```text
邮箱 + 密码 + Turnstile（若启用）
  -> 登录
  -> 普通用户：获得登录凭证
  -> 启用 2FA 的用户：获得 temp_token
       -> 提交 temp_token + 身份验证器动态码
       -> 获得登录凭证
```

邮箱验证码是注册验证，不是邮箱验证码登录功能。密码登录接口不接收 `verify_code`。

## 3. 获取注册配置

```bash
curl 'https://YOUR_DOMAIN/api/v1/settings/public'
```

响应节选，其他字段省略：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "registration_enabled": true,
    "email_verify_enabled": true,
    "registration_email_suffix_whitelist": [],
    "invitation_code_enabled": false,
    "promo_code_enabled": false,
    "totp_enabled": true,
    "turnstile_enabled": false,
    "turnstile_site_key": "",
    "backend_mode_enabled": false
  }
}
```

| 字段 | 下游处理方式 |
| --- | --- |
| `registration_enabled` | 为 false 时不提供新用户注册 |
| `email_verify_enabled` | 为 true 时必须先发送邮件验证码，再携带验证码注册 |
| `registration_email_suffix_whitelist` | 非空时限制注册邮箱后缀，服务端执行最终校验 |
| `invitation_code_enabled` | 控制邀请码功能；当前邮箱注册实现中，未填写邀请码仍可注册，填写后才校验 |
| `promo_code_enabled` | 控制注册优惠码功能 |
| `totp_enabled` | 平台是否开启 TOTP 功能，不表示当前用户一定需要 2FA；以登录响应为准 |
| `turnstile_enabled` / `turnstile_site_key` | 下游前端获取人机验证 token，传入相应接口；不要向下游暴露服务端 Secret Key |
| `backend_mode_enabled` | 后端模式下普通客户认证/访问可能被限制 |

若公开配置包含启用的 `login_agreement_enabled`、`login_agreement_mode`、`login_agreement_documents` 等字段，下游应按平台要求展示登录注册协议。本文的邮箱注册请求没有额外的协议确认字段。

## 4. 邮箱验证注册

### 4.1 发送验证码

```bash
curl -X POST 'https://YOUR_DOMAIN/api/v1/auth/send-verify-code' \
  -H 'Content-Type: application/json' \
  -H 'Accept-Language: zh-CN' \
  -d '{"email":"customer@example.com"}'
```

| 请求字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `email` | string | 是 | 有效且未注册的邮箱 |
| `turnstile_token` | string | 条件必填 | 启用 Turnstile 时，填写前端新取得的有效 token |

启用 Turnstile 时的请求体示例：

```json
{
  "email": "customer@example.com",
  "turnstile_token": "TOKEN_FROM_TURNSTILE_WIDGET"
}
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "Verification code sent successfully",
    "countdown": 60
  }
}
```

- 邮箱验证码为 6 位数字，有效期 15 分钟，使用字符串传输以保留前导零。
- 同一邮箱发送冷却期当前为 60 秒，客户端根据 `data.countdown` 倒计时。
- 注册校验累计输错达到 5 次会返回 `VERIFY_CODE_MAX_ATTEMPTS`，需要重新申请验证码。
- 验证通过后验证码会被消费。若后续注册失败，不应假定原验证码还能继续使用。
- 接口不会返回邮箱验证码。发送会等待邮件投递调用结果，成功不等于收件箱必然已经收到邮件。
- Turnstile token 是一次性的，重新发送或再次登录时应重新取得 token。

### 4.2 提交注册

```bash
curl -X POST 'https://YOUR_DOMAIN/api/v1/auth/register' \
  -H 'Content-Type: application/json' \
  -d '{
    "email":"customer@example.com",
    "password":"ExamplePassword123!",
    "verify_code":"123456"
  }'
```

| 请求字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `email` | string | 是 | 与发送验证码时相同的邮箱 |
| `password` | string | 是 | 至少 6 位 |
| `verify_code` | string | 条件必填 | 开启邮箱验证时必填，来自真实邮件 |
| `turnstile_token` | string | 条件必填 | 未启用邮箱验证但启用 Turnstile 时需要；邮箱验证开启且提交验证码时不重复校验发送阶段的 token |
| `invitation_code` | string | 否 | 可选邀请码，开启相应功能且传入时校验 |
| `promo_code` | string | 否 | 注册优惠码，按平台配置处理 |
| `aff_code` | string | 否 | 邀请返利码，按平台配置处理 |

注册成功响应节选：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "ACCESS_TOKEN_FROM_SERVER",
    "refresh_token": "rt_REFRESH_TOKEN_FROM_SERVER",
    "expires_in": 3600,
    "token_type": "Bearer",
    "user": {
      "id": 101,
      "email": "customer@example.com"
    }
  }
}
```

注册成功已返回登录凭证，无需再次调用登录接口。`expires_in` 单位是秒，`3600` 只是示例，应以实际返回值为准。当前实现有仅返回 Access Token 的降级分支，`refresh_token`、`expires_in` 可能缺省；没有刷新令牌时，凭证失效后需要重新登录。

注册请求超时不代表注册失败。可先尝试登录确认账户是否已经建立，避免反复重放注册请求或消费验证码。

## 5. 登录与 2FA

### 5.1 密码登录

```bash
curl -X POST 'https://YOUR_DOMAIN/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"email":"customer@example.com","password":"ExamplePassword123!"}'
```

必填字段为 `email`、`password`；启用 Turnstile 时增加 `turnstile_token`。普通登录成功响应与注册成功结构相同。

需要 2FA 时也返回 HTTP 200，但响应中没有最终登录凭证：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "requires_2fa": true,
    "temp_token": "TEMP_TOKEN_FROM_SERVER",
    "user_email_masked": "c***@example.com"
  }
}
```

先判断 `data.requires_2fa === true`，不要仅凭 HTTP 200 就视为已经登录。`temp_token` 不能用作账户接口的 Bearer Token。

### 5.2 完成 2FA

```bash
curl -X POST 'https://YOUR_DOMAIN/api/v1/auth/login/2fa' \
  -H 'Content-Type: application/json' \
  -d '{"temp_token":"TEMP_TOKEN_FROM_SERVER","totp_code":"123456"}'
```

`temp_token`、`totp_code` 均为必填字符串，动态码长度为 6。`totp_code` 来自用户绑定的身份验证器，不是注册邮件中的验证码。

成功响应与注册成功结构相同。临时会话无效或过期时当前接口返回 HTTP 400，可能只有 `message`；应重新走密码登录流程。

## 6. 用户信息与凭证管理

### 6.1 查询当前用户

```bash
curl 'https://YOUR_DOMAIN/api/v1/auth/me' \
  -H 'Authorization: Bearer ACCESS_TOKEN_FROM_SERVER'
```

响应节选：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 101,
    "email": "customer@example.com",
    "run_mode": "standard"
  }
}
```

注意 `/auth/me` 的用户字段直接位于 `data` 中，而注册/登录响应的用户字段位于 `data.user` 中。

### 6.2 刷新凭证

```bash
curl -X POST 'https://YOUR_DOMAIN/api/v1/auth/refresh' \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"rt_REFRESH_TOKEN_FROM_SERVER"}'
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "NEW_ACCESS_TOKEN",
    "refresh_token": "rt_NEW_REFRESH_TOKEN",
    "expires_in": 3600,
    "token_type": "Bearer"
  }
}
```

每次刷新都会轮换令牌，客户端应同时替换 Access Token 和 Refresh Token。旧刷新令牌按设计被消费，不要复用或并发刷新。同一会话的多个请求应共用一次刷新结果。

遇到刷新令牌无效、过期或已撤销时，清除本地凭证并重新登录。刷新网络超时可能发生在旧令牌已消费之后，不能无限重试旧令牌。刷新响应不包含 `user`，需要时另查 `/auth/me`。

### 6.3 退出登录

```bash
curl -X POST 'https://YOUR_DOMAIN/api/v1/auth/logout' \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"rt_NEW_REFRESH_TOKEN"}'
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {"message":"Logged out successfully"}
}
```

客户端应清除本地登录状态和两种令牌。该接口尝试撤销提交的 Refresh Token，不能据此认为已签发的 Access Token 立即失效；撤销失败当前也可能返回成功。若不传 `refresh_token`，不能据此保证刷新令牌已被撤销。

如需主动撤销当前用户全部会话，可使用 `POST /api/v1/auth/revoke-all-sessions`，携带当前有效登录 JWT，无需请求体。

### 6.4 登录后接入账户功能

使用登录 JWT 调用：

- `GET /api/v1/user/balance`：客户展示余额。
- `GET /api/v1/keys/defaults`：获取默认模型 API 密钥。
- `POST /api/v1/redeem`：兑换兑换码。

详细说明见 [客户账户 API](customer-account-api.md) 和 [默认 API 密钥](default-api-keys.md)。登录 JWT 与默认模型 API 密钥用途不同，应分别保存和使用，避免将密码、验证码或完整令牌写入日志。

## 7. 错误与限流

| HTTP 状态 | 常见 reason | 下游处理 |
| --- | --- | --- |
| 400 | 可能缺省 | 请求参数不合法，检查 JSON、邮箱、密码、2FA 临时会话 |
| 400 | `EMAIL_VERIFY_REQUIRED` | 注册时补充邮箱验证码 |
| 400 | `INVALID_VERIFY_CODE` | 验证码无效或过期，检查输入或重新发送 |
| 400 | `EMAIL_SUFFIX_NOT_ALLOWED` / `EMAIL_RESERVED` | 更换符合平台规则的邮箱 |
| 400 | `INVITATION_CODE_INVALID` | 检查填写的邀请码 |
| 400 | `TURNSTILE_VERIFICATION_FAILED` | 重新完成人机验证 |
| 401 | `INVALID_CREDENTIALS` | 邮箱或密码不正确 |
| 401 | `REFRESH_TOKEN_INVALID` / `REFRESH_TOKEN_EXPIRED` / `TOKEN_REVOKED` | 清除失效凭证，重新登录 |
| 401 | 可能缺省或其他认证 reason | 登录 JWT 缺失/失效，按认证响应处理 |
| 403 | `REGISTRATION_DISABLED` | 平台未开放注册 |
| 403 | `USER_NOT_ACTIVE` | 账户被停用 |
| 403 | `BACKEND_MODE_ADMIN_ONLY` 或缺省 | 平台模式限制普通客户访问 |
| 409 | `EMAIL_EXISTS` | 邮箱已注册，引导登录 |
| 429 | `VERIFY_CODE_TOO_FREQUENT` | 等待邮件验证码冷却后重试 |
| 429 | `VERIFY_CODE_MAX_ATTEMPTS` | 错误次数过多，重新申请验证码 |
| 429 | 缺省 | 触发路由限流，或限流依赖异常时拒绝请求 |
| 503 | `TURNSTILE_NOT_CONFIGURED` / `SERVICE_UNAVAILABLE` | 平台配置或依赖异常，联系平台维护人员 |
| 5xx | 可能缺省 | 服务异常；不要将内部错误原文直接暴露给终端用户 |

表中为常见错误，不是穷举；客户端应保留 HTTP 状态和可用的 `reason`，并对未知错误兜底。

当前以下限制分别按接口、按平台识别的客户端 IP 计数：

| 接口 | 每分钟请求上限 |
| --- | --- |
| `/auth/send-verify-code` | 5 |
| `/auth/register` | 5 |
| `/auth/login` | 20 |
| `/auth/login/2fa` | 20 |
| `/auth/refresh` | 30 |

验证码还受同一邮箱 60 秒冷却限制。路由限流依赖 Redis，依赖故障时也会拒绝请求。路由层 429 响应格式如下，没有 `code` 和 `reason`，也不能假定一定有 `Retry-After` 响应头：

```json
{
  "error": "rate limit exceeded",
  "message": "Too many requests, please try again later"
}
```

## 8. 可执行接口模拟测试样例

### 8.1 运行方式与边界

环境要求：Node.js 22 或更新版本。以下命令可在 macOS/Linux 终端直接运行，无需安装第三方包。

样例启动仅绑定 `127.0.0.1` 的临时 HTTP Mock 服务，用真实 HTTP 请求校验请求方法、路径、JSON 和 Authorization，再检查客户端对固定响应的处理。执行结束自动关闭服务；不会创建真实账户或发送邮件。

这些是显式编写的接口响应夹具，不是生产认证服务：不实现密码哈希、验证码过期、JWT 签名、真实刷新撤销、数据库、Redis、SMTP、Turnstile 或 CORS。负向结果由测试夹具指定，不能证明线上后端已经通过这些测试。

### 8.2 完整可运行示例

```bash
node --input-type=module <<'NODE'
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';

const user = { id: 101, email: 'customer@example.com' };
const credentials = { email: user.email, password: 'MockPassword123!' };
const auth = (suffix) => ({
  access_token: `mock_access_${suffix}`,
  refresh_token: `rt_mock_${suffix}`,
  expires_in: 3600,
  token_type: 'Bearer',
  user,
});
const ok = (data) => ({ code: 0, message: 'success', data });
const failure = (code, reason, message) => ({ code, reason, message });
const cases = [];
function fixture(name, method, path, body, status, response, authorization) {
  cases.push({ name, method, path: `/api/v1${path}`, body, status,
    response, authorization });
}

fixture('public settings', 'GET', '/settings/public', undefined, 200, ok({
  registration_enabled: true, email_verify_enabled: true,
  turnstile_enabled: false, invitation_code_enabled: false,
}));
fixture('send code', 'POST', '/auth/send-verify-code', { email: user.email },
  200, ok({ message: 'Verification code sent successfully', countdown: 60 }));
fixture('email cooldown', 'POST', '/auth/send-verify-code', { email: user.email },
  429, failure(429, 'VERIFY_CODE_TOO_FREQUENT', 'please wait before requesting a new code'));
fixture('missing code', 'POST', '/auth/register', credentials,
  400, failure(400, 'EMAIL_VERIFY_REQUIRED', 'email verification is required'));
fixture('invalid code', 'POST', '/auth/register', { ...credentials, verify_code: '000000' },
  400, failure(400, 'INVALID_VERIFY_CODE', 'invalid or expired verification code'));
fixture('register', 'POST', '/auth/register', { ...credentials, verify_code: '123456' },
  200, ok(auth('register')));
fixture('current user', 'GET', '/auth/me', undefined,
  200, ok({ ...user, run_mode: 'standard' }), 'Bearer mock_access_register');
fixture('normal login', 'POST', '/auth/login', credentials, 200, ok(auth('login')));
fixture('wrong password', 'POST', '/auth/login', { ...credentials, password: 'wrong' },
  401, failure(401, 'INVALID_CREDENTIALS', 'invalid email or password'));
// This fixture represents a later login after the user has enabled TOTP.
fixture('2fa challenge', 'POST', '/auth/login', credentials,
  200, ok({ requires_2fa: true, temp_token: 'mock_temp', user_email_masked: 'c***@example.com' }));
fixture('2fa completion', 'POST', '/auth/login/2fa',
  { temp_token: 'mock_temp', totp_code: '654321' }, 200, ok(auth('2fa')));
const { user: omittedUser, ...refreshed } = auth('refreshed');
fixture('refresh', 'POST', '/auth/refresh', { refresh_token: 'rt_mock_2fa' },
  200, ok(refreshed));
fixture('old refresh token', 'POST', '/auth/refresh', { refresh_token: 'rt_mock_2fa' },
  401, failure(401, 'REFRESH_TOKEN_INVALID', 'invalid refresh token'));
fixture('logout', 'POST', '/auth/logout', { refresh_token: 'rt_mock_refreshed' },
  200, ok({ message: 'Logged out successfully' }));
fixture('revoked refresh token', 'POST', '/auth/refresh', { refresh_token: 'rt_mock_refreshed' },
  401, failure(401, 'REFRESH_TOKEN_INVALID', 'invalid refresh token'));
fixture('route rate limit', 'POST', '/auth/login', credentials, 429,
  { error: 'rate limit exceeded', message: 'Too many requests, please try again later' });

let cursor = 0;
const serverErrors = [];
const server = createServer(async (req, res) => {
  try {
    const expected = cases[cursor++];
    assert.ok(expected, 'Unexpected extra HTTP request');
    assert.equal(req.method, expected.method);
    assert.equal(req.url, expected.path);
    assert.equal(req.headers.authorization, expected.authorization);
    let raw = '';
    for await (const chunk of req) raw += chunk;
    if (expected.body !== undefined) {
      assert.equal(req.headers['content-type'], 'application/json');
    }
    assert.deepEqual(raw ? JSON.parse(raw) : undefined, expected.body);
    res.writeHead(expected.status, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(expected.response));
  } catch (error) {
    serverErrors.push(error);
    res.writeHead(500, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ code: 500, message: 'Mock request assertion failed' }));
  }
});

server.listen(0, '127.0.0.1');
await once(server, 'listening');
const base = `http://127.0.0.1:${server.address().port}/api/v1`;

async function api(method, path, body, token) {
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (token) headers.Authorization = `Bearer ${token}`;
  const response = await fetch(base + path, {
    method, headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(5000),
  });
  const raw = await response.text();
  let envelope;
  try { envelope = JSON.parse(raw); } catch { envelope = {}; }
  if (!response.ok || envelope.code !== 0) {
    const error = new Error(envelope.message || `HTTP ${response.status}`);
    error.status = response.status;
    error.reason = envelope.reason;
    throw error;
  }
  return envelope.data;
}
async function expectFailure(method, path, body, status, reason) {
  await assert.rejects(() => api(method, path, body), (error) => {
    assert.equal(error.status, status);
    assert.equal(error.reason, reason);
    return true;
  });
}

try {
  const settings = await api('GET', '/settings/public');
  assert.equal(settings.email_verify_enabled, true);
  const sent = await api('POST', '/auth/send-verify-code', { email: user.email });
  assert.equal(sent.countdown, 60);
  await expectFailure('POST', '/auth/send-verify-code', { email: user.email },
    429, 'VERIFY_CODE_TOO_FREQUENT');
  await expectFailure('POST', '/auth/register', credentials, 400, 'EMAIL_VERIFY_REQUIRED');
  await expectFailure('POST', '/auth/register', { ...credentials, verify_code: '000000' },
    400, 'INVALID_VERIFY_CODE');
  let session = await api('POST', '/auth/register', { ...credentials, verify_code: '123456' });
  assert.equal(session.access_token, 'mock_access_register');
  assert.equal(session.user.id, user.id);
  const me = await api('GET', '/auth/me', undefined, session.access_token);
  assert.equal(me.id, user.id);
  session = await api('POST', '/auth/login', credentials);
  assert.equal(session.access_token, 'mock_access_login');
  await expectFailure('POST', '/auth/login', { ...credentials, password: 'wrong' },
    401, 'INVALID_CREDENTIALS');
  const challenge = await api('POST', '/auth/login', credentials);
  assert.equal(challenge.requires_2fa, true);
  assert.equal(challenge.access_token, undefined);
  session = await api('POST', '/auth/login/2fa', {
    temp_token: challenge.temp_token, totp_code: '654321',
  });
  assert.equal(session.access_token, 'mock_access_2fa');
  const oldRefresh = session.refresh_token;
  const tokens = await api('POST', '/auth/refresh', { refresh_token: oldRefresh });
  assert.notEqual(tokens.refresh_token, oldRefresh);
  assert.equal(tokens.access_token, 'mock_access_refreshed');
  assert.equal(tokens.user, undefined);
  session = { ...session, ...tokens };
  await expectFailure('POST', '/auth/refresh', { refresh_token: oldRefresh },
    401, 'REFRESH_TOKEN_INVALID');
  const latestRefresh = session.refresh_token;
  const loggedOut = await api('POST', '/auth/logout', { refresh_token: latestRefresh });
  assert.equal(loggedOut.message, 'Logged out successfully');
  session = null;
  assert.equal(session, null);
  await expectFailure('POST', '/auth/refresh', { refresh_token: latestRefresh },
    401, 'REFRESH_TOKEN_INVALID');
  await expectFailure('POST', '/auth/login', credentials, 429, undefined);
  assert.equal(serverErrors.length, 0, serverErrors[0]?.message);
  assert.equal(cursor, cases.length);
  for (const item of cases) console.log(`PASS ${item.name}`);
  console.log(`PASS ${cases.length} HTTP mock cases; no live backend was tested.`);
} finally {
  const closed = new Promise((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
  });
  server.closeAllConnections();
  await closed;
}
NODE
```

成功时最后一行输出：

```text
PASS 16 HTTP mock cases; no live backend was tested.
```

### 8.3 真实环境联调步骤

1. 使用测试环境域名，调用公开配置，确认注册、邮箱验证、Turnstile 和平台模式。
2. 使用能收信、符合后缀规则的未注册邮箱调用发送验证码接口。若启用 Turnstile，先从前端取得真实 token。
3. 使用邮件中的验证码调用注册接口，检查 `data.access_token`、`data.user`，记录可用的刷新令牌。
4. 用注册返回的 JWT 调用 `/auth/me`，确认返回同一客户身份。
5. 用该邮箱和密码登录。为已启用 TOTP 的测试用户另外执行 2FA 分支。
6. 刷新当前会话，保存新令牌；退出后清除本地状态，并确认该刷新令牌无法继续刷新。
7. 在测试环境分别检查验证码错误/过期、邮箱重复、密码错误、Turnstile 失败及 429 处理。邮件验证码有消费与错误次数限制，各用例应独立准备数据。

这些调用会真实发送邮件、创建账户和改变会话状态。Mock 中的 `123456`、`654321` 和 `mock_*` 值仅用于模拟，不可用于真实鉴权。不要只更换 Mock 脚本中的地址就当作真实联调脚本，其固定断言和负向响应需要独立的测试数据与条件。

## 9. 实现依据与发布说明

- [认证路由与限流](../backend/internal/server/routes/auth.go)
- [认证请求、响应和处理器](../backend/internal/handler/auth_handler.go)
- [注册、Turnstile、邀请码及刷新逻辑](../backend/internal/service/auth_service.go)
- [邮件验证码与冷却逻辑](../backend/internal/service/email_service.go)
- [公开配置](../backend/internal/handler/setting_handler.go)
- [路由限流响应格式](../backend/internal/middleware/rate_limiter.go)

本文为仓库内对接文档；上线时需同步到后台 `doc_url` 对应的文档站。字段与规则应以实际部署版本为准。本文附带 Mock 样例验证，不代表真实 SMTP、Turnstile 或线上账户链路已经完成联调。
