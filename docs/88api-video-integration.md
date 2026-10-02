# 88API Video 接入与试用

平台 ID：`88api-video`。上游默认地址：`https://88api.ai/v1`。

部署时需包含 `backend/migrations/199_user_platform_quotas_add_88api.sql`，并按项目既有迁移流程启动新版后端；本次未对实际数据库执行迁移。

## 账户配置

1. 在账户管理创建 `88API Video` API Key 账户，填写上游密钥。
2. 点击同步上游模型，复用 `POST /api/v1/admin/accounts/models/sync-upstream-preview`。已保存账户使用 `POST /api/v1/admin/accounts/{id}/models/sync-upstream`。
3. 后端请求上游 `GET /v1/models`，读取实际返回模型，合并至表单，保存账户后生效。同步失败保留原配置。
4. 选择需要提供的视频模型，绑定同平台分组。同步不使用本地固定目录；接口只有模型名称时，不以名称猜测并删除未知型号。若密钥包含文本/图片模型，管理员须取消这些模型的选择。

同步按按钮触发；未新增后台定时同步。新模型支持通用 `/v1/videos` 协议时可配置使用，特殊字段和能力仍以上游契约为准。

## 定价

在渠道管理添加 `88API Video` 平台，关联上述分组，给精确模型名称配置“视频”计费。

- 动态分辨率模型：按 `480p`、`720p`、`768p`、`1080p`、`2k`、`4k` 添加分辨率层级，每层分别填写每秒单价。
- 已锁定分辨率的模型，如 `SD2.5 720P`、`kling-3.0-turbo-4k`：可直接填写该精确模型的每秒价格，也可配置对应层级。
- `Seedance-2.0-720p官方版` 与 `Seedance-2.0-fast-720p官方版` 同样属于固定 720p 模型。定价模型名匹配与渠道系统一致，不区分大小写，但不同版本、`fast`、分辨率后缀仍须完整区分；发给上游的模型名不因此改写。
- 层级缺失、模型未配置、只有通配符价格或价格属于其他平台时，生成请求被拒绝，不回退到分组统一价格或默认图片价格。
- 示例：同一个模型 480p 配置 0.1/s、720p 配置 0.4/s，8 秒基础费用分别为 0.8、3.2，再应用已有分组/账户倍率。示例数字不是上游报价。
- 冻结和结算复用 PP Video 任务。价格在提交时快照保存；查询不再次扣费。88API 文档未承诺实际计费时长字段，本接入按明确请求的输出秒数结算，不从媒体播放时长推算费用。

若仍报 `configure 88API video pricing`，错误末尾会包含实际 `group_id` 和原因：未匹配到渠道模型价格、计费模式不对、只匹配通配符、缺少固定模型默认价格、缺少对应分辨率层级，或价格为空/非法。配置了层级的规则仍要求命中对应层级；不会因默认价存在而静默忽略缺价层级。

## 下游请求

下游使用本系统密钥，该密钥须绑定 `88api-video` 分组。

```http
POST /v1/videos/generations
Authorization: Bearer <本系统密钥>
Idempotency-Key: <每次新生成的唯一标识；重试复用同一个标识>
Content-Type: application/json
```

```json
{
  "model": "SD2.5 720P",
  "prompt": "海岸日出，镜头缓慢推进",
  "duration": 8,
  "size": "16:9",
  "images": ["https://your-cdn.example/reference.jpg"],
  "generate_audio": true
}
```

动态分辨率模型使用 `metadata.resolution`，例如 `grok-imagine-video` 配合 `"metadata":{"resolution":"720p"}`。本接口也接受顶层 `resolution` 并规范化。

仅 `88api-video` 对以下精确模型名（不区分大小写）强制使用固定分辨率：

| 分辨率 | 模型 |
| --- | --- |
| 480p | `SD2.0 480P`、`SD2.5 480P`、`seedance-2.0-mini-480p`、`wan3.0-video-480p` |
| 720p | `SD2.0 720P`、`SD2.5 720P`、`Seedance-2.0-720p官方版`、`Seedance-2.0-fast-720p官方版`、`Seedance-2.5-720p官方版`、`kling-3.0-turbo-720p`、`seedance-2.0-mini-720p`、`wan3.0-video-720p` |
| 1080p | `SD2.0 1080P`、`SD2.5 1080P`、`grok-imagine-video-1.5-1080p`、`kling-3.0-turbo-1080p`、`wan3.0-video-1080p` |
| 2k | `kling-3.0-turbo-2k` |
| 4k | `kling-3.0-turbo-4k` |

固定模型忽略顶层 `resolution`、`parameters.resolution`、`metadata.resolution`，不因它们与模型档位不一致而报错；转发时通过模型名确定档位，并保留画面比例。已支持的像素尺寸 `size` 转换为对应画面比例，避免覆盖固定档位。请求规范化、计费及返回的 `usage.resolution` 均采用固定分辨率。

其余模型（包括 Gemini Omni、无固定档位的 Grok、Veo、`minimax-h3-768p` 和新同步模型）采用下游传入的分辨率，不根据模型后缀自动锁定。目前可识别 `480p`、`720p`、`768p`、`1080p`、`2k`、`4k`；仍需配置对应渠道价格，且最终生成能力以上游为准。模型同步仍为在线同步，上表不是账号模型白名单。

88API 同时兼容既有下游的 `input + parameters` 格式，客户无需改为顶层字段：

```json
{
  "model": "grok-imagine-video",
  "input": {
    "prompt": "海岸日出，镜头缓慢推进",
    "img_url": "https://your-cdn.example/reference.jpg"
  },
  "parameters": {
    "duration": 5,
    "resolution": "720p",
    "aspect_ratio": "9:16"
  }
}
```

仅在 `88api-video` 分支，将 `input.prompt`、`input.img_url`、`parameters.duration`、`parameters.resolution`、`parameters.aspect_ratio` 分别转换为 `prompt`、`images`、`duration`、`metadata.resolution`、`size`。文生视频可以省略 `img_url`。转换发生在参数校验及计费之前，示例按 720p、5 秒报价；相同内容的两种格式规范化后具有相同请求哈希。

不要在两种格式里重复指定同一目标字段，或同时填写 `parameters.duration` 与 `seconds`；固定模型被忽略的分辨率字段除外。动态模型多个分辨率字段冲突仍会报错。图片仍须为原始 HTTPS URL，签名参数原样保留，不接受 Markdown 链接包装，也不会修复过期签名。模型时长和参考素材校验不因格式转换而放宽。

返回 `id` 后调用 `GET /v1/videos/{id}`，状态为 `processing`、`succeeded` 或 `failed`。当前沿用已有 PP Video 契约，公开返回 88API 的任务 ID；内部 `local_task_id` 用于冻结和幂等，不是对外查询 ID。查询按用户及 API Key 验证任务归属，并使用原账户轮询。

成功时读取 `url` / `video_url` / `result_url`，完整保留签名参数。媒体下载不发送上游 Key；不拼接上游 `/content` 地址。88API 上传素材和归档视频通常保存 30 天，官方直链以自身有效期为准。

## 支持边界

- 首版使用轮询，拒绝非空 `callback_url`，不提供取消、上传凭证代理或自动归档。
- 每任务仅一条视频，时长必须显式填写，`duration` 与 `seconds` 二选一。
- 已知型号校验时长、比例、参考数量、首尾帧组合及音频开关；未公开的 H3 能力由上游校验。
- Veo 显式图片模式要求 PNG/JPEG Base64；普通视频/音频参考要求匿名公网 HTTPS 地址。
- `queued` / `in_progress` / `unknown` 保持处理中。仅 `completed` 且有 HTTPS 结果地址才成功；进度 100 或预览 URL 不代表成功。
- 已获得上游 ID 的任务不会仅因本地等待超时而退款；持续轮询原任务。需关注长期未终态任务及冻结金额。
- 提交超时且未取得上游 ID 时不能断定上游未受理。现有提交失败流程释放本地冻结，并保留幂等记录；不要换幂等标识盲目重发，须先向上游核对。

## 验证范围

本次验证使用模拟上游覆盖模型同步、创建/查询路径和鉴权、响应状态、参数校验、模型分辨率价格及长任务轮询。未使用真实 API Key 进行收费生成。真实上线前应使用目标账户验证在线模型响应和各模型族的最小生成请求。

- 新增 `Test88API*` 后端测试全部通过；网关处理器与路由编译通过。
- 前端 6 个测试文件、111 项测试通过，类型检查通过。
- 扩大回归发现 7 项既有失败，已在未修改的 HEAD 基线上复现；未修改这些无关测试或行为。
- 真实数据库迁移、余额冻结/退款的数据库集成测试及真实模型生成尚未验证。
