# 站点媒体生成接口

本文档是本站对下游客户提供的媒体生成接口文档，覆盖视频生成和 Qwen TTS 语音合成。下游只需要对接本站接口，不需要关心后台使用的具体上游服务商或其凭据。

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

同一个 `Idempotency-Key` 重试同一个请求会返回同一个任务结果；同一个 key 不能用于不同请求。

## 音频语音合成（Qwen TTS）

Qwen TTS 使用同步接口生成音频。API Key 必须属于平台为 `qwen-tts` 的分组；管理员配置的 ModelVerse 凭据不会返回给调用方。

### 创建语音

`POST /v1/audio/speech`

```bash
curl https://YOUR_SITE/v1/audio/speech \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3-tts-flash",
    "input": "今天天气真好，我们去公园散步吧。",
    "voice": "Cherry",
    "metadata": {"language_type": "Chinese"}
  }'
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `model` | 是 | 从该 Qwen TTS 账号同步到的上游支持模型中选择；可通过 `GET /v1/models` 查询。 |
| `input` | 是 | 要合成的文本，最长 600 个字符。 |
| `voice` | 是 | 上游音色名称，例如 `Cherry`、`Ethan`、`Serena`、`Chelsie`。 |
| `metadata.language_type` | 否 | `Auto`（默认）、`Chinese`、`English`、`German`、`Italian`、`Portuguese`、`Spanish`、`Japanese`、`Korean`、`French` 或 `Russian`。 |

成功时会原样返回 Qwen TTS 的响应，其中包含临时音频地址：

```json
{
  "request_id": "5c63c65c-cad8-4bf4-959d-example",
  "code": "",
  "message": "",
  "output": {
    "finish_reason": "stop",
    "audio": {
      "data": "",
      "url": "https://example.com/audio.wav",
      "id": "audio_5c63c65c-cad8-4bf4-959d-example",
      "expires_at": 1766113409
    }
  },
  "usage": {
    "input_tokens": 0,
    "output_tokens": 0,
    "characters": 195
  }
}
```

### 音频计费

Qwen TTS 使用渠道配置的 Token 计费方式。网关以响应中的 `usage.characters` 作为本次请求的输入 Token 数量；上游文档中的 `input_tokens` 与 `output_tokens` 均为 `0`，不会参与计费。上游请求失败或业务返回失败时不会生成用量扣费。

上游返回的音频 URL 有有效期，请在过期前下载或持久化音频。

## 视频生成

### 查询可用模型

推荐先通过模型列表查询当前 API Key 可用的视频模型：

```bash
curl https://YOUR_SITE/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

从响应 `data[].id` 中选择一个模型填入生成请求的 `model`。模型列表只返回当前
API Key 所属视频平台同步到的模型，不同平台的模型不能混用。

如果请求未传 `model`，本站会根据 API Key 所属的视频平台和可用账号模型映射，
自动选择该平台的实际模型。下游不应把即梦模型
`by-seedance2.0-933` 作为 Kling、Happy Horse 或 PP Seedance 的模型使用。

`video-v1` 仍作为历史文档兼容模型名接受，但推荐始终使用 `/v1/models` 返回的
精确模型 ID。本站会按 API Key 所属分组和后台账号配置，把对外模型名转换为对应
上游请求。

#### 新增视频模型

以下模型通过本文档中的统一创建和查询接口调用。API Key 必须属于已开通对应视频平台的分组；不同平台的模型不能跨分组使用。下游应以 `/v1/models` 返回的结果为准，并使用其中的精确模型 ID，不要使用 `video-v1` 作为新接入的模型名。

| 平台 | 模型 ID | 输入能力 | 时长 | 输出分辨率 |
| --- | --- | --- | --- | --- |
| ByteDance | `doubao-seedance-2-0-260128` | 文生、首尾帧、参考图片/视频/音频 | 4-15 秒整数 | `480p`、`720p`、`1080p`、`4K` |
| Wan3.0 | `wan3.0-video`、`wan3.0-video-prime` | 文生、首尾帧、参考图片/视频/音频、文件或链接参考 | 2-30 秒整数 | `480P`、`720P`、`1080P` |
| MiniMax-H3 | `MiniMax-H3` | 文生、首尾帧、参考图片/视频/音频 | 4-15 秒整数 | `768P`、`2K` |
| MiniMax-H3 | `MiniMax-Hailuo-2.3` | 文生、图生（首帧图片） | 6 或 10 秒整数 | `768P`、`1080P`（仅 6 秒） |
| Pixverse-V6 | `pixverse-v6` | 文生、首尾帧、参考图、视频延长 | 1-15 秒整数 | `360p`、`540p`、`720p`、`1080p` |
| Grok Imagine Video | `grok-imagine-video` | 图生、参考生 | 1-15 秒整数 | `480p`、`720p` |

### 创建视频任务

推荐接口：

```http
POST /v1/videos/generations
```

兼容旧接口：

```http
POST /v1/video/generations
```

请求提交后立即返回任务 ID。视频任务固定异步执行，随后请按“查询任务状态”章节轮询结果。

#### 请求字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 否 | 对外视频模型名，建议从 `/v1/models` 获取。未传时按该 API Key 所属平台和账号模型映射自动选择。 |
| `prompt` | string | 条件必填 | 视频描述。`prompt`、参考图字段或首尾帧字段至少提供一项。 |
| `duration` | integer/number/string | 否 | 视频时长，单位秒。建议传 `5`、`10` 或 `15`；未传默认 `5`。不同模型的最小/最大时长以“新增视频模型参数”章节为准。 |
| `images` | string[] | 否 | 推荐的参考图片 URL 列表。多张图片必须是同一人物、同一角色或同一视觉风格的补充参考；不要混入不同人物或不同次元/画风的图片。 |
| `image` | string | 否 | 单张参考图片的兼容写法。不能与 `images`、`image_url` 同时传；Seedance 会将其标准化为 `images`。 |
| `image_url` | string | 否 | 单张参考图片 URL 的兼容写法。不能与 `images`、`image` 同时传；Seedance 会将其标准化为 `images`。 |
| `videos` | string[] | 否 | 参考视频 URL 列表，会透传给支持该能力的上游。 |
| `audios` | string[] | 否 | 参考音频 URL 列表，会透传给支持该能力的上游。 |
| `aspect_ratio` | string | 否 | 画幅比例，例如 `16:9`、`9:16`、`1:1`。 |
| `resolution` | string | 否 | 分辨率，例如 `480p`、`720p`、`1080p`；未传默认 `720p`。 |
| `mode` | string | 否 | 视频模式。K-Ling 可传 `std`、`pro`、`4k`，也兼容 `2x`、`2x-pro`；未传时本站按 `resolution` 自动映射。 |
| `generate_audio` | boolean | 否 | 是否生成视频音频，以模型实际上游能力为准。 |
| `input_video_duration` | number | 否 | 参考视频时长，单位秒。会随请求透传给支持该能力的上游；不参与本站视频费用计算。 |
| `output_width` | integer | 否 | 输出宽度，单位像素。会随请求透传给支持该能力的上游；不参与本站视频费用计算。 |
| `output_height` | integer | 否 | 输出高度，单位像素。会随请求透传给支持该能力的上游；不参与本站视频费用计算。 |
| `fps` | number | 否 | 输出帧率。会随请求透传给支持该能力的上游；不参与本站视频费用计算。 |
| `start_frame_url` | string | 否 | 首帧图片 URL，适用于支持首尾帧控制的模型。Seedance 中不能与 `images`、`image`、`image_url` 同时传。 |
| `end_frame_url` | string | 否 | 尾帧图片 URL，适用于支持首尾帧控制的模型。Seedance 中不能与 `images`、`image`、`image_url` 同时传。 |
| `seed` | integer | 否 | 随机种子。 |
| `n` | integer | 否 | 生成视频数量，默认 `1`。 |

不同模型对参考图、参考视频、参考音频、首尾帧、时长、分辨率和音频生成的支持不完全相同。建议使用可公开访问的 HTTPS 资源 URL，并根据目标模型能力传入相应字段。K-Ling 的 `duration` 仅支持 `3` 到 `15` 秒的整数值；本站会把统一字段自动转换为 K-Ling 上游需要的 `model_name`、`image`、`image_tail`、`sound` 等字段。

本文档开头列出的 ByteDance、Wan3.0、MiniMax-H3/Hailuo 和 Pixverse-V6 模型也兼容上述部分统一字段，便于已有下游迁移；新接入时应优先使用下一章节的模型专用结构。本站不会透出对应上游的任务提交、状态查询地址或上游 API Key。

### 新增视频模型参数

所有模型仍使用统一的 `POST /v1/videos/generations` 创建任务和 `GET /v1/videos/{task_id}` 查询任务状态。以下 JSON 结构是新增视频模型推荐使用的请求体格式。

#### ByteDance

模型 ID 固定为 `doubao-seedance-2-0-260128`。`input.content` 至少包含一个内容项，可使用 `text`、`image_url`、`video_url`、`audio_url` 四种类型：

- `text`：`{"type":"text","text":"提示词"}`。
- `image_url`：`{"type":"image_url","image_url":{"url":"https://..."}, "role":"first_frame|last_frame|reference_image"}`。
- `video_url`：`{"type":"video_url","video_url":{"url":"https://..."}, "role":"reference_video"}`。
- `audio_url`：`{"type":"audio_url","audio_url":{"url":"https://..."}, "role":"reference_audio"}`。

`parameters.duration` 必须是 `4` 到 `15` 的整数；`parameters.resolution` 可为 `480p`、`720p`、`1080p` 或 `4K`；`parameters.ratio` 可省略，省略时为 `adaptive`。可选参数包括 `generate_audio`（boolean）、`seed`、`camera_fixed`、`watermark`、`callback_url`、`execution_expires_after` 和 `seedance_tools`。

```json
{
  "model": "doubao-seedance-2-0-260128",
  "input": {
    "content": [
      {"type": "text", "text": "清晨的海边，一艘小船穿过薄雾，电影感镜头"},
      {
        "type": "image_url",
        "image_url": {"url": "https://example.com/first-frame.jpg"},
        "role": "first_frame"
      }
    ]
  },
  "parameters": {
    "duration": 5,
    "resolution": "720p",
    "ratio": "16:9",
    "generate_audio": false
  }
}
```

#### Wan3.0

模型为 `wan3.0-video` 或 `wan3.0-video-prime`。`input.prompt` 必填；`input.media` 可省略，或传入以下对象组成的数组：

```json
{"type": "first_frame|last_frame|reference_image|reference_video|reference_audio|file|link", "url": "https://..."}
```

首帧和尾帧各最多一张；参考图最多 10 张，参考视频和参考音频各最多 5 个；`file` 和 `link` 最多二选一。`first_frame`/`last_frame` 不能与参考媒体、`file` 或 `link` 混用。图片类媒体支持公开 URL 或 `data:image/...;base64,...`，其余媒体使用可访问的 URL。

`parameters.duration` 必须是 `2` 到 `30` 的整数；`parameters.resolution` 可为 `480P`、`720P` 或 `1080P`，默认 `1080P`；`parameters.ratio` 可为 `adaptive`、`16:9`、`9:16`、`1:1`、`4:3` 或 `3:4`。可选参数包括 `audio`、`prompt_extend`、`watermark` 和 `seed`（`0` 至 `2147483647` 的整数）。

```json
{
  "model": "wan3.0-video",
  "input": {
    "prompt": "猫咪从草地左边跑到右边，阳光明媚，镜头跟拍",
    "media": [
      {"type": "first_frame", "url": "https://example.com/start.jpg"},
      {"type": "last_frame", "url": "https://example.com/end.jpg"}
    ]
  },
  "parameters": {
    "duration": 8,
    "resolution": "720P",
    "ratio": "16:9",
    "audio": true,
    "prompt_extend": true
  }
}
```

#### MiniMax-H3

模型 ID 固定为 `MiniMax-H3`。`input.content` 必填，长度为 1 到 16 项，且必须且只能包含一个非空 `text` 项。其余媒体项使用与 ByteDance 相同的对象形式：

- 图片：`image_url`，`role` 为 `first_frame`、`last_frame` 或 `reference_image`。
- 视频：`video_url`，`role` 固定为 `reference_video`。
- 音频：`audio_url`，`role` 固定为 `reference_audio`。

首帧和尾帧各最多一张，参考图最多 9 张，参考视频和参考音频各最多 3 个。首尾帧模式不能与参考媒体混用；只传参考音频无效，必须同时有参考图或参考视频。媒体仅支持可公开访问的 HTTP/HTTPS URL，不支持 Base64。

`parameters.duration` 必须是 `4` 到 `15` 的整数；`parameters.resolution` 为 `768P` 或 `2K`。`parameters.ratio` 可为 `adaptive`、`21:9`、`16:9`、`4:3`、`1:1`、`3:4` 或 `9:16`：文生视频不能使用 `adaptive`，首尾帧模式按 `adaptive` 处理。可选参数 `aigc_watermark` 为 boolean，默认 `false`。

```json
{
  "model": "MiniMax-H3",
  "input": {
    "content": [
      {"type": "text", "text": "水墨画从静止的山水渐变为云雾流动的动态场景"},
      {
        "type": "image_url",
        "image_url": {"url": "https://example.com/first-frame.png"},
        "role": "first_frame"
      },
      {
        "type": "image_url",
        "image_url": {"url": "https://example.com/last-frame.png"},
        "role": "last_frame"
      }
    ]
  },
  "parameters": {
    "duration": 10,
    "resolution": "2K",
    "ratio": "adaptive",
    "aigc_watermark": false
  }
}
```

#### MiniMax-Hailuo-2.3

模型 ID 固定为 `MiniMax-Hailuo-2.3`，与 `MiniMax-H3` 共用后台的 MiniMax-H3 平台账号，但请求参数和输入结构相互独立。

- 文生视频：`input.prompt` 必填。
- 图生视频：在 `input` 中增加 `first_frame_image`，支持公开 HTTP/HTTPS 图片 URL 或 `data:image/...;base64,...`。
- `input.prompt` 最长 2000 个字符；源图片格式、大小、尺寸和宽高比由上游进一步校验。
- `parameters.duration` 只能是 `6` 或 `10`；`6` 秒支持 `768P`、`1080P`，`10` 秒仅支持 `768P`。
- `parameters.resolution` 可为 `768P` 或 `1080P`。可选参数 `prompt_optimizer` 默认 `true`、`fast_pretreatment` 默认 `false`、`aigc_watermark` 默认 `false`。

文生视频请求：

```json
{
  "model": "MiniMax-Hailuo-2.3",
  "input": {
    "prompt": "A beautiful sunset over the ocean with waves gently crashing on the shore. [推进, 跟随]"
  },
  "parameters": {
    "duration": 6,
    "resolution": "1080P",
    "prompt_optimizer": true,
    "fast_pretreatment": false,
    "aigc_watermark": false
  }
}
```

图生视频请求：

```json
{
  "model": "MiniMax-Hailuo-2.3",
  "input": {
    "first_frame_image": "https://example.com/first-frame.jpg",
    "prompt": "A mouse runs toward the camera, smiling and blinking. [推进, 跟随]"
  },
  "parameters": {
    "duration": 10,
    "resolution": "768P"
  }
}
```

#### Pixverse-V6

模型 ID 固定为 `pixverse-v6`，`input.prompt` 必填。可选输入为：

- `input.first_frame_url` 和 `input.last_frame_url`：首尾帧必须同时提供，支持公开图片 URL 或 `data:image/...;base64,...`。
- `input.img_url`：参考图，支持公开图片 URL 或 `data:image/...;base64,...`。
- `input.video_url`：视频延长输入，仅支持公开 HTTP/HTTPS URL。

`parameters.duration` 必须是 `1` 到 `15` 的整数；`parameters.resolution` 可为 `360p`、`540p`、`720p` 或 `1080p`，默认 `720p`。纯文生视频可传 `parameters.aspect_ratio`，可选 `16:9`、`4:3`、`1:1`、`3:4`、`9:16`、`2:3`、`3:2` 或 `21:9`；带任一图片或视频输入时，该字段不会生效。`generate_audio` 为整数音频开关，默认 `1`（with_audio，有声音），传 `0` 可关闭声音；`seed` 可选，取值范围为 `0` 至 `2147483647`。

```json
{
  "model": "pixverse-v6",
  "input": {
    "prompt": "一只纸鹤从书桌上起飞，穿过窗外的晚霞",
    "img_url": "https://example.com/reference.jpg"
  },
  "parameters": {
    "duration": 5,
    "resolution": "720p",
    "generate_audio": 1
  }
}
```

#### Grok Imagine Video

模型 ID 固定为 `grok-imagine-video`，仅支持图生和参考生视频，不支持纯文生视频。`input.prompt` 必填，并且必须在以下两种输入中二选一：

- `input.img_url`：单张参考图片的 HTTP/HTTPS URL。
- `input.reference_urls`：一张或多张参考图片的 HTTP/HTTPS URL 数组。

二者不能同时传。`parameters.duration` 为 `1` 至 `15` 的整秒；`parameters.resolution` 仅支持 `480p`、`720p`；`parameters.aspect_ratio` 支持 `1:1`、`16:9`、`9:16`、`4:3`、`3:4`、`3:2`、`2:3`。

```json
{
  "model": "grok-imagine-video",
  "input": {
    "prompt": "让画面中的纸鹤振翅飞过晚霞",
    "img_url": "https://example.com/reference.png"
  },
  "parameters": {
    "duration": 5,
    "resolution": "720p",
    "aspect_ratio": "9:16"
  }
}
```

#### 图片输入规则

Seedance 请求必须在下面两种模式中二选一：

- **参考图模式**：使用 `images`（推荐）或兼容字段 `image`、`image_url` 之一。三个字段不能混传。`images` 中的多张图应是同一角色的不同角度或细节补充；图片顺序不代表人物身份或画风的优先级，也不保证锁定人物身份。
- **首尾帧模式**：使用 `start_frame_url` 和可选的 `end_frame_url` 控制视频首尾画面。该模式不能同时传任何参考图字段。

这项限制用于避免上游在“人物参考”和“首帧/尾帧控制”之间自行选择主输入，从而造成角色身份或 3D/真人画风漂移。若目标是保持人物一致性，请只传同一角色的参考图，并在 `prompt` 中明确要求保持角色身份、材质和画风。

#### 文生视频示例

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

#### 带参考素材示例

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

#### 创建响应

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

### 查询任务状态

推荐接口：

```http
GET /v1/videos/{task_id}
```

兼容旧接口：

```http
GET /v1/video/generations/{task_id}
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

### 下载或播放视频

任务成功后返回的 `video_url` 是本站或上游媒体代理地址，可能不会以 `.mp4` 结尾。客户端应根据 HTTP `Content-Type` 或播放器的媒体探测能力处理视频，不要通过文件扩展名判断格式。

部分上游（尤其是 Wan3.0）返回的是带 OSS 签名参数的临时 URL。签名参数中的
`Signature` 可能包含标准 Base64 字符 `+`、`/` 和 `=`，客户端或媒体代理必须保留
URL 的完整内容：

- 如果把视频 URL 放入代理接口的查询参数，必须对完整 URL 进行一次 URL 编码，例如
  JavaScript 使用 `encodeURIComponent(videoURL)`；不要直接拼接
  `?url=`。
- 服务端只解码外层参数一次，不要对已经解码的完整 URL 再次 `QueryEscape` 或重新
  拼接其签名查询参数。
- 更推荐使用 POST JSON 传递 `{ "url": "..." }`，避免嵌套查询参数解析时把
  `+` 误解为空格。
- 该 URL 具有有效期，过期后应重新查询任务状态获取新的 `video_url`，不要缓存旧的
  OSS 签名 URL。

### 计费与退款

视频生成在本站按统一的站内规则计费，具体上游服务商的接口和计费实现不会暴露给下游客户。上游服务商更换、模型映射调整或接口路径变化，不要求下游客户修改本文档中的调用方式。

本站当前处理规则：

- 创建任务时按预计用量预占余额；任务成功后按实际生成结果结算。
- 任务失败、取消或超时会释放预占金额，失败请求不产生视频费用。
- 任务成功但上游没有返回实际输出时长时，按提交时的输出时长结算。
- 具体计费模型如下：

| 平台 | 计费规则 |
| --- | --- |
| Seedance | 按输出视频秒数计费，单价取本站该 API Key 所属分组的原有视频价格配置。`480p`、`720p`、`1080p` 分别使用对应价格，`4K` 使用 `1080p` 价格。 |
| K-Ling | 按输出视频秒数计费，单价取本站该 API Key 所属分组的原有视频价格配置。2x 使用 `720p` 价格，2x Pro 和 4K 使用 `1080p` 价格。 |
| Happy Horse | 按输出视频秒数计费，720p/1080p 分别取本站该 API Key 所属分组的 `720p`/`1080p` 视频价格。 |
| ByteDance | 按输出视频秒数计费。`480p`、`720p`、`1080p` 分别使用分组对应价档，`4K` 使用 `1080p` 价档。 |
| Wan3.0 | 按输出视频秒数计费。`480P`、`720P`、`1080P` 分别使用分组对应价档。 |
| MiniMax-H3 | 按输出视频秒数计费。`768P` 使用分组 `720p` 价档，`2K` 使用分组 `1080p` 价档。 |
| MiniMax-Hailuo-2.3 | 按输出视频秒数计费。`768P` 使用分组 `720p` 价档，`1080P` 使用分组 `1080p` 价档。 |
| Pixverse-V6 | 按输出视频秒数计费。`360p` 使用分组 `480p` 价档；`540p` 和 `720p` 使用分组 `720p` 价档；`1080p` 使用分组 `1080p` 价档。 |

- K-Ling 的价格档位由请求中的 `mode` 决定；音频参数不改变本站的站内单价。
- 上述基础费用还会叠加本站分组倍率、视频独立倍率（如果启用）和账号倍率。
- `videos`、`audios`、`input_video_duration`、输出宽高和帧率等字段会按上游能力透传，但不会改变本站的按秒计费结果。

### Python 轮询示例

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

### 常见错误

| HTTP 状态 | 场景 | 处理方式 |
| --- | --- | --- |
| `400` | 请求参数无效、没有有效输入、时长或分辨率不支持 | 检查请求字段，至少传入 `prompt`、`image` 或 `images` 之一。 |
| `401` | API Key 缺失或无效 | 检查 `Authorization`。 |
| `402` | 余额或额度不足 | 充值或调整生成参数。 |
| `404` | 当前 API Key 分组不支持视频接口，或任务不存在 | 检查 API Key 分组和任务 ID。 |
| `409` | `Idempotency-Key` 被不同请求复用 | 使用新的唯一 key。 |
| `502` | 上游服务调用失败 | 稍后重试或调整模型能力范围内的参数。 |
