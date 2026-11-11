# 站点视频生成接口

本文档是本站对下游客户提供的视频生成接口文档。下游使用已开通对应平台的本站 API Key 调用本站接口，无需直接对接上游地址或持有上游凭据。语音合成见 [Qwen TTS 语音合成接口](QWEN_TTS_API.md)。

## 先看这里：怎么选 v1 还是 v2

创建视频任务有两种写法，使用同一个 API Key，查询任务和下载视频的接口完全一样：

| 写法 | 创建接口 | 请求体 | 适合谁 |
| --- | --- | --- | --- |
| v2（推荐） | `POST /v2/videos/generations` | 所有模型用同一种请求体，素材统一放在 `assets[]` | 新接入的客户 |
| v1 | `POST /v1/videos/generations` | 按模型各写各的，写法见侧边栏“各模型原生写法（v1）” | 已经接入的客户，或暂不支持 v2 的模型 |

接入步骤：

1. 用你的 API Key 调用 `GET /v1/models`，选一个模型。每个模型都带 `capabilities` 字段，其中 `protocols` 写明它支持 `v1`，还是 `v1` 和 `v2`，另外还有时长、分辨率、画幅、素材数量的范围。
2. `protocols` 包含 `v2` 的，按 [统一请求格式（v2）](video-api/unified.md) 调用 `POST /v2/videos/generations`；只有 `v1` 的，按该模型自己的页面调用 `POST /v1/videos/generations`。
3. 创建成功后保存响应里的任务 ID，用 `GET /v1/videos/{task_id}` 轮询状态，完成后按“下载或播放视频”一节取得视频。创建时建议带 `Idempotency-Key`。

已经在用 v1 的客户不需要迁移，v1 会继续可用。

## 基础信息

```text
Base URL: https://YOUR_SITE
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

请将 `YOUR_SITE` 替换为本站域名或地址，将 `YOUR_API_KEY` 替换为已创建的 API Key。

创建视频任务建议额外携带：

```http
Idempotency-Key: YOUR_UNIQUE_REQUEST_KEY
```

同一个 `Idempotency-Key` 重试同一个请求会返回同一个任务结果；同一个 key 不能用于不同请求。该请求头是可选的：不传时每次调用都会创建独立的视频任务，即使请求正文完全相同。

## 1. 选择模型

推荐先通过模型列表查询当前 API Key 可用的视频模型：

```bash
curl https://YOUR_SITE/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

从响应 `data[].id` 中选择一个模型填入生成请求的 `model`。模型列表只返回当前
API Key 所属视频平台同步到的模型，不同平台的模型不能混用。

每个视频模型还会带有 `capabilities` 字段，说明它支持的请求写法、时长、分辨率、画幅、素材数量等，字段说明见 [统一请求格式（v2）](video-api/unified.md) 的“查询模型能力”一节。

如果请求未传 `model`，本站会根据 API Key 所属的视频平台自动选择该平台的实际模型。不同平台的模型不能混用。

### 模型总览

以下模型通过本文档中的统一创建和查询接口调用。API Key 必须属于已开通对应视频平台的分组；不同平台的模型不能跨分组使用，**Seedance 综合分组**除外：它把多条 Seedance 线路放在同一个分组里，一个 API Key 可以调用其中全部线路，见 [Seedance 综合分组](video-api/seedance-hub.md)。下游应以 `/v1/models` 返回的结果为准，并使用其中的精确模型 ID。

| 模型系列 | 模型 ID | 输入能力 | 时长 | 输出分辨率 | 调用说明 |
| --- | --- | --- | --- | --- | --- |
| Seedance（doubao 系列） | `doubao-seedance-2-0-260128`、`doubao-seedance-2-5-260628`、`doubao-seedance-2-5-260628-global` | 文生、首尾帧、参考图片/视频/音频 | 2.0：4-15 秒整数；2.5：4-30 秒整数 | 2.0：`480p`、`720p`、`1080p`、`4K`；2.5：`480p`、`720p`、`1080p` | [查看](video-api/doubao-seedance.md) |
| Wan3.0 | `wan3.0-video`、`wan3.0-video-prime` | 文生、首尾帧、参考图片/视频/音频、文件或链接参考 | 2-30 秒整数 | `480P`、`720P`、`1080P` | [查看](video-api/wan.md) |
| 综合视频模型 | `gemini-omni-flash`、`gemini-omni-flash-1.1`、`grok-imagine-video`、`grok-imagine-video-1.5`、`kling-3.0`、`seedance-2.0`、`seedance-2.0-fast`、`seedance-2.0-mini`、`seedance-2.5`、`veo-3.1`、`veo-3.1-fast`、`wan3.0-video` | 文生、图生及参考素材能力因模型而异 | 1-30 秒，按模型限制 | `480p`、`720p`、`1080p`、`4K`，按模型限制 | [查看](video-api/general-models.md) |
| Seedance（其他型号） | `seedance-2.0-930`、`seedance-2.0-903`、`seedance-933`、`seedance2.0-933-C`、`seedance2.5-V`、`Seedance 2.5-全参720P`、`限时限量-Seedance 2.5 满参`、`Seedance 2.5-无视频`、`限时限量-Seedance 2.5 满参-备选`、`Seedance 2.0按次计费`、`Seedance 2.5-无参考视频 按秒计费`、`Seedance 2.0-无参考视频 按秒计费`、`竖屏-Seedance 2.0按次计费`、`竖屏-Seedance 2.0 Fast 按次计费`、`竖屏-Seedance 2.0 Mini 按次计费` | 文生、参考图片/视频/音频，部分模型支持首尾帧，按模型能力 | 4-30 秒，按模型限制 | `480p`、`720p`、`1080p`，按模型限制 | [查看](video-api/seedance-series.md) |
| Seedance 2.5 480P | `Seedance2.5-480全参` | 文生、首尾帧、参考图片/视频/音频 | 4-30 秒整数 | 固定 `480p` | [查看](video-api/seedance-series.md) |
| Seedance 综合分组 | 名称形如 `Seedance2.5--720p--V--按次--X`，全部线路见调用说明 | 文生、参考图片/视频/音频，部分线路支持首尾帧 | 4-30 秒，按线路限制 | `480p`、`720p`、`1080p`，按线路限制 | [查看](video-api/seedance-hub.md) |
| MiniMax-H3 | `MiniMax-H3`、`minimax-h3-lite` | 文生、首尾帧、参考图片/视频/音频 | 4-30 秒整数 | `480P`、`768P`、`1080P`、`2K`、`4K` | [查看](video-api/minimax-h3.md) |
| Pixverse-V6 | `pixverse-v6` | 文生、首尾帧、参考图、视频延长 | 1-15 秒整数 | `360p`、`540p`、`720p`、`1080p` | [查看](video-api/pixverse.md) |

所有模型仍使用统一的 `POST /v1/videos/generations` 创建任务和 `GET /v1/videos/{task_id}` 查询任务状态。各模型调用说明中的 JSON 结构是新增视频模型推荐使用的请求体格式。

### 各模型请求写法速查

不同模型的原生请求写法并不相同，下表汇总了主要差异，方便同时接入多个模型时对照。表中未列出的型号以各自的调用说明为准。

| 模型系列 | 素材的写法 | 时长、分辨率、画幅 | 音频开关 |
| --- | --- | --- | --- |
| Seedance（doubao 系列） | `input.content[]`，元素为 `image_url` / `video_url` / `audio_url`，用 `role` 区分首帧、尾帧和参考 | `parameters.*` | `parameters.generate_audio` |
| Wan3.0 | `input.media[]`，元素为 `{type, url}` | `parameters.*` | `parameters.audio` |
| 综合视频模型 | 顶层 `images`、`videos`；参考音频放 `metadata.referenceAudios` | 顶层 `duration`、`resolution`、`ratio` | 顶层 `generate_audio`，仅部分型号支持 |
| Seedance（其他型号） | 顶层 `images`、`videos`、`audios`；名称带中文的型号还可以用带 `name` 的 `assets[]` | 顶层 `duration`、`resolution`、`aspect_ratio` 或 `ratio` | 以各型号说明为准 |
| Seedance2.5-480全参 | 顶层 `referenceImages`、`referenceVideos`、`referenceAudios`、`first_image`、`last_image` | 顶层 `duration`、固定 `resolution`、`ratio` | 顶层 `generate_audio` |
| MiniMax-H3 | 顶层 `content[]` | 顶层 `duration`、`resolution`、`ratio` | `mute_audio`，传 `true` 表示关闭音频 |
| Pixverse-V6 | `input.img_url`、`input.first_frame_url`、`input.last_frame_url`、`input.video_url` | `parameters.*` | `parameters.generate_audio` |

如果不想为每个模型各写一套请求，可以使用 [统一请求格式（v2）](video-api/unified.md)：所有素材放在 `assets[]`，时长、分辨率、画幅和音频开关使用固定字段名。目前本节模型总览中的全部模型都支持统一请求格式：Seedance（doubao 系列）、Wan3.0、MiniMax-H3、Pixverse-V6、综合视频模型、Seedance（其他型号）（包括 `seedance-2.0-930`、`seedance-2.0-903`）、`Seedance2.5-480全参` 以及 [Seedance 综合分组](video-api/seedance-hub.md) 里的全部线路。个别写法仍只能用原生写法，例如 Wan3.0 的文件、链接参考和 `Seedance2.5-480全参` 的首帧图片文件上传，见各模型的调用说明。

## 2. 创建视频任务（v1 写法）

本节是 v1 写法。新接入推荐使用 [统一请求格式（v2）](video-api/unified.md)，所有模型用同一种请求体。

推荐接口：

```http
POST /v1/videos/generations
```

请求提交后立即返回任务 ID。视频任务固定异步执行，随后请按“查询任务状态”章节轮询结果。

### 请求字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 否 | 对外视频模型名，建议从 `/v1/models` 获取。未传时按该 API Key 所属平台和账号模型映射自动选择。 |
| `prompt` | string | 条件必填 | 视频描述。`prompt`、参考图字段或首尾帧字段至少提供一项。 |
| `duration` | integer/number/string | 否 | 视频时长，单位秒。建议传 `5`、`10` 或 `15`；未传默认 `5`。不同模型的最小/最大时长以各模型的调用说明为准。 |
| `images` | string[] | 否 | 推荐的参考图片 URL 列表。多张图片必须是同一人物、同一角色或同一视觉风格的补充参考；不要混入不同人物或不同次元/画风的图片。 |
| `videos` | string[] | 否 | 参考视频 URL 列表，会透传给支持该能力的上游。 |
| `audios` | string[] | 否 | 参考音频 URL 列表，会透传给支持该能力的上游。 |
| `assets` | object[] | 否 | 带名称的参考素材列表（`type`、`url`、`name`），仅部分 Seedance 型号支持。需要在提示词中用 `@文件名` 引用素材时使用，详见 [Seedance（其他型号）](video-api/seedance-series.md) 中的“用 assets[] 传素材并指定名称”一节。 |
| `aspect_ratio` | string | 否 | 画幅比例，例如 `16:9`、`9:16`、`1:1`。 |
| `resolution` | string | 否 | 分辨率，例如 `480p`、`720p`、`1080p`；未传默认 `720p`。 |
| `generate_audio` | boolean | 否 | 是否生成视频音频，以模型实际能力为准；传 `false` 可关闭。 |
| `n` | integer | 否 | 生成视频数量，默认 `1`。 |

不同模型对参考图、参考视频、参考音频、首尾帧、时长、分辨率和音频生成的支持不完全相同。建议使用可公开访问的 HTTPS 资源 URL，并根据目标模型能力传入相应字段。

模型总览中列出的 Seedance（doubao 系列）、Wan3.0、综合视频模型、MiniMax-H3 和 Pixverse-V6 模型也兼容上述部分统一字段，便于已有下游迁移；新接入时应优先使用各模型调用说明中的模型专用结构。本站不会透出对应上游的任务提交、状态查询地址或上游 API Key。

MiniMax-H3 以其 [调用说明](video-api/minimax-h3.md) 为准：默认分辨率为 `768P`，`duration` 使用 JSON 整数，媒体通过 `content` 传入。不要直接套用上表中的 `images`、`videos`、`audios` 或 `generate_audio` 字段。

首尾帧、种子、兼容写法等更多字段见 [进阶说明](video-api/advanced.md)。

### 文生视频示例

```bash
curl -X POST https://YOUR_SITE/v1/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: video-demo-001" \
  -d '{
    "prompt": "清晨的海边，一艘小船穿过薄雾，电影感镜头",
    "duration": 5,
    "aspect_ratio": "16:9",
    "resolution": "720p",
    "generate_audio": true
  }'
```

### 带参考素材示例

以下示例使用 `assets[]` 传参考素材，并为每个素材指定 `name`，提示词中可以用 `@文件名` 引用。该写法适用于名称带中文的 Seedance 型号（如 `Seedance 2.5-全参720P`，完整说明见 [Seedance（其他型号）](video-api/seedance-series.md) 中的“用 assets[] 传素材并指定名称”一节）。`name` 必须与提示词里的 `@` 引用完全一致，同一请求内不能重复。

```bash
curl -X POST https://YOUR_SITE/v1/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Seedance 2.5-全参720P",
    "prompt": "@角色.png 在 @场景.png 中自然向前走并回头微笑，配音参考 @配音.wav",
    "duration": 5,
    "resolution": "720p",
    "aspect_ratio": "9:16",
    "assets": [
      { "type": "image", "name": "角色.png", "url": "https://example.com/character.png" },
      { "type": "image", "name": "场景.png", "url": "https://example.com/scene.png" },
      { "type": "audio", "name": "配音.wav", "url": "https://example.com/voice.wav" }
    ]
  }'
```

其他不支持 `assets[]` 的模型，使用 `images`、`videos`、`audios` 传参考素材。这三个字段只传 URL，不带文件名，本站会自动命名为 `image1`、`video1`、`audio1` 等：

```bash
curl -X POST https://YOUR_SITE/v1/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "保持参考素材中的人物和整体氛围，人物自然向前走并回头微笑",
    "duration": 5,
    "images": ["https://example.com/reference-image.jpg"],
    "videos": ["https://example.com/reference-video.mp4"],
    "audios": ["https://example.com/reference-audio.mp3"],
    "aspect_ratio": "9:16",
    "generate_audio": true
  }'
```

### 创建响应

```json
{
  "id": "task_xxxxxxxxxxxx",
  "task_id": "task_xxxxxxxxxxxx",
  "object": "video.generation.task",
  "status": "processing",
  "model": "实际使用的当前平台模型 ID",
  "created_at": 1786690000
}
```

`id` 和 `task_id` 含义相同，都是后续查询任务使用的任务 ID。创建响应也可能返回 `submitted` 或上游原始状态，客户端应继续轮询直到成功或失败。

## 3. 查询任务状态

推荐接口：

```http
GET /v1/videos/{task_id}
```

示例：

```bash
curl https://YOUR_SITE/v1/videos/task_xxxxxxxxxxxx \
  -H "Authorization: Bearer YOUR_API_KEY"
```

成功响应示例：

```json
{
  "id": "task_xxxxxxxxxxxx",
  "task_id": "task_xxxxxxxxxxxx",
  "object": "video.generation.task",
  "status": "succeeded",
  "model": "实际使用的当前平台模型 ID",
  "created_at": 1786690000,
  "completed_at": 1786690060,
  "video_url": "https://YOUR_SITE/v1/video/assets/asset_xxxxxxxxxxxx",
  "data": {
    "video_url": "https://YOUR_SITE/v1/video/assets/asset_xxxxxxxxxxxx"
  },
  "usage": {
    "duration_ms": 5041,
    "duration_seconds": 5.041,
    "video_count": 1,
    "resolution": "720p"
  }
}
```

任务通常会经历 `submitted`、`processing`、`succeeded` 或 `failed` 等状态。请间隔数秒轮询，直到任务成功或失败。
客户端不需要保持创建请求的长连接；创建接口返回任务 ID 后，应改为轮询查询接口。不要仅凭固定等待时间（例如 20 分钟）判定任务失败，应以查询接口返回的 `succeeded` 或 `failed` 为准；业务侧如需设置最大等待时间，建议按自身体验要求配置，并在超时后继续保留任务 ID 以便稍后查询。

| 状态 | 说明 |
| --- | --- |
| `submitted` | 任务已提交。 |
| `processing` | 任务排队中或生成中。 |
| `succeeded` | 任务成功完成。 |
| `failed` | 任务失败、取消、超时或被上游拒绝。 |

响应中可能包含额外上游字段，客户应只依赖本文档列出的稳定字段。

## 4. 下载或播放视频

任务成功后返回的 `video_url` 是本站或上游媒体代理地址，可能不会以 `.mp4` 结尾。客户端应根据 HTTP `Content-Type` 或播放器的媒体探测能力处理视频，不要通过文件扩展名判断格式。

签名 URL 的注意事项见 [进阶说明](video-api/advanced.md)。

## 5. 完整示例（Python）

```python
import time
import requests

base_url = "https://YOUR_SITE"
headers = {
    "Authorization": "Bearer YOUR_API_KEY",
    "Content-Type": "application/json",
}

create = requests.post(
    f"{base_url}/v1/videos/generations",
    headers=headers,
    json={
        "prompt": "雨后城市街道，镜头缓慢推进",
        "duration": 5,
    },
    timeout=30,
)
create.raise_for_status()
task_id = create.json()["id"]

while True:
    response = requests.get(
        f"{base_url}/v1/videos/{task_id}",
        headers=headers,
        timeout=30,
    )
    response.raise_for_status()
    task = response.json()
    status = str(task.get("status", "")).lower()

    if status in {"succeeded", "success", "completed"}:
        data = task.get("data")
        result = data if isinstance(data, dict) else task
        print(result.get("video_url") or task.get("video_url") or result.get("result_url"))
        break

    if status in {"failed", "failure", "error", "cancelled", "canceled"}:
        raise RuntimeError(task)

    time.sleep(5)
```

## 6. 计费与退款

视频生成按本站公布的价格计费，价格以积分计，具体单价请在控制台「模型与价格」页面查看；下游无需使用上游计价。本站创建和查询接口保持统一；切换平台分组时，仍需核对对应平台的模型、参数和价格。

本站当前处理规则：

- 创建任务时按预计用量预占余额；任务成功后按实际生成结果结算。
- 任务失败、取消或超时会释放预占金额，失败请求不产生视频费用。
- 任务成功但上游没有返回实际输出时长时，按提交时的输出时长结算。
- 不同模型按输出视频秒数计费（按秒）或按次计费（按次），采用哪种方式以及各分辨率档位的单价，以「模型与价格」页面为准。
- MiniMax-H3 请求 `1080P`、`2K` 或 `4K` 后若实际降级为 `768P`，按 `768P` 的价格结算。创建任务时会锁定所选档位的价格，之后的价格调整不影响进行中的任务；无需由下游传入价格。
- 取消请求返回 `running` 时不会立即释放预占金额，最终取消或失败后才释放。
- `videos`、`audios`、`input_video_duration`、输出宽高和帧率等字段会按上游能力透传，但不会改变本站的按秒计费结果。
- 若所选模型或分辨率档位暂未开通价格，创建请求会被拒绝，请联系本站。
- 实际扣费以任务创建时的报价和使用记录为准。

## 7. 常见错误

| HTTP 状态 | 场景 | 处理方式 |
| --- | --- | --- |
| `400` | 请求参数无效、没有有效输入、时长或分辨率不支持 | 检查请求字段，至少传入 `prompt`、`image` 或 `images` 之一。 |
| `401` | API Key 缺失或无效 | 检查 `Authorization`。 |
| `402` | 余额或额度不足 | 充值或调整生成参数。 |
| `404` | 当前 API Key 分组不支持视频接口，或任务不存在 | 检查 API Key 分组和任务 ID。 |
| `409` | `Idempotency-Key` 被不同请求复用 | 使用新的唯一 key。 |
| `502` | 上游服务调用失败 | 稍后重试或调整模型能力范围内的参数。 |

## 8. 更多文档

- [Seedance（doubao 系列）](video-api/doubao-seedance.md)
- [Seedance（其他型号）](video-api/seedance-series.md)
- [Wan3.0](video-api/wan.md)
- [综合视频模型](video-api/general-models.md)
- [MiniMax-H3](video-api/minimax-h3.md)（含取消任务）
- [Pixverse-V6](video-api/pixverse.md)
- [进阶说明](video-api/advanced.md)（兼容旧接口、更多请求字段、图片输入规则、签名 URL）
- [Qwen TTS 语音合成接口](QWEN_TTS_API.md)
