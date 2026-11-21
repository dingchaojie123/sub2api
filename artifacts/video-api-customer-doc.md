# 视频生成接口接入文档（精简版）

本文档说明如何调用本站的视频生成接口。你只需要使用已开通对应视频平台的本站 API Key 调用本站接口，无需对接上游地址，也不需要持有上游凭据。

## 1. 快速开始

### 1.1 基础信息

```text
Base URL: https://YOUR_SITE
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

请将 `YOUR_SITE` 替换为本站域名或地址，将 `YOUR_API_KEY` 替换为你在本站创建的 API Key。

创建任务时建议额外携带：

```http
Idempotency-Key: YOUR_UNIQUE_REQUEST_KEY
```

同一个 `Idempotency-Key` 重试同一个请求会返回同一个任务结果；同一个 key 不能用于不同请求。该请求头是可选的：不传时每次调用都会创建独立的视频任务，即使请求正文完全相同。

### 1.2 接入三步

1. 调用 `GET /v1/models`，选一个模型。每个模型都带有 `capabilities` 字段，写明它支持的请求写法、时长、分辨率、画幅和素材数量（见第 4 节）。
2. 调用 `POST /v2/videos/generations` 创建任务，所有模型使用同一种请求体，素材统一放在 `assets[]`（见第 3 节）。
3. 创建成功后保存响应里的任务 ID，用 `GET /v1/videos/{task_id}` 轮询状态，任务成功后取得视频（见第 6、7 节）。

## 2. 选择模型

先查询当前 API Key 可用的视频模型：

```bash
curl https://YOUR_SITE/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

从响应 `data[].id` 中选择一个模型，填入创建请求的 `model`。模型列表只返回当前 API Key 所属视频平台的模型，不同平台的模型不能混用。以 `/v1/models` 返回的结果为准，并使用其中的精确模型 ID。

如果请求未传 `model`，本站会根据 API Key 所属的视频平台自动选择该平台的实际模型。

### 模型总览

| 模型系列 | 模型 ID | 输入能力 | 时长 | 输出分辨率 |
| --- | --- | --- | --- | --- |
| Seedance（doubao 系列） | `doubao-seedance-2-0-260128`、`doubao-seedance-2-5-260628`、`doubao-seedance-2-5-260628-global` | 文生、首尾帧、参考图片/视频/音频 | 2.0：4-15 秒整数；2.5：4-30 秒整数 | 2.0：`480p`、`720p`、`1080p`、`4K`；2.5：`480p`、`720p`、`1080p` |
| Wan3.0（c3 型号） | `wan3.0-video-c3` | 文生、首尾帧、参考图片/视频/音频 | 2-30 秒整数 | `480p`、`720p`、`1080p`；默认 `1080p` |
| Seedance（其他型号） | `seedance-2.0-930`、`seedance-2.0-903`、`seedance-2.0-933`、`seedance-933`、`seedance2.0-933-C`、`seedance2.5-V`、`Seedance 2.5-全参720P`、`限时限量-Seedance 2.5 满参`、`Seedance 2.5-无视频`、`限时限量-Seedance 2.5 满参-备选`、`Seedance 2.0按次计费`、`Seedance 2.5-无参考视频 按秒计费`、`Seedance 2.0-无参考视频 按秒计费`、`竖屏-Seedance 2.0按次计费`、`竖屏-Seedance 2.0 Fast 按次计费`、`竖屏-Seedance 2.0 Mini 按次计费` | 文生、参考图片/视频/音频，部分模型支持首尾帧，按模型能力 | 4-30 秒，按模型限制 | `480p`、`720p`、`1080p`，按模型限制 |
| Seedance 2.5 480P | `Seedance2.5-480全参` | 文生、首尾帧、参考图片/视频/音频 | 4-30 秒整数 | 固定 `480p` |
| Seedance 综合分组 | 名称形如 `Seedance2.5--720p--V--按次--X`，全部线路以 `/v1/models` 返回为准 | 文生、参考图片/视频/音频，部分线路支持首尾帧 | 4-30 秒，按线路限制 | `480p`、`720p`、`1080p`，按线路限制 |
| MiniMax-H3 | `MiniMax-H3`、`minimax-h3-lite` | 文生、首尾帧、参考图片/视频/音频 | 4-30 秒整数 | `480p`、`768p`、`1080p`、`2k`、`4k` |

API Key 必须属于已开通对应视频平台的分组，不同平台的模型不能跨分组使用。**Seedance 综合分组**除外：它把多条 Seedance 线路放在同一个分组里，一个 API Key 可以调用其中全部线路。

## 3. 创建视频任务（统一请求格式）

```http
POST /v2/videos/generations
```

不同视频模型各自的原生请求写法并不相同。统一请求格式让你用同一组字段调用上面表中的所有模型，由本站负责转换成各模型需要的写法。

请求体必须是 `application/json`。认证方式和 `Idempotency-Key` 的用法见 1.1。每个请求只生成一个视频，不接受 `n`。请求提交后立即返回任务 ID，视频任务固定异步执行。

### 3.1 请求字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 否 | 模型名，从 `GET /v1/models` 获取。 |
| `prompt` | string | 条件必填 | 提示词。Seedance（doubao 系列）的 `prompt` 和 `assets` 至少提供一项；名称带中文的 Seedance 型号必须提供 `prompt`。 |
| `duration` | integer | 否 | 视频时长，单位秒，只接受整数，不接受字符串或小数。可选范围由所选模型决定。 |
| `resolution` | string | 否 | 分辨率，例如 `720p`。可选值以所选模型为准。 |
| `ratio` | string | 否 | 画幅比例，例如 `16:9`、`9:16`。不接受 `aspect_ratio`、`size`。 |
| `generate_audio` | boolean | 否 | 是否生成音频，仅部分模型支持，见 3.3。`true` 表示生成音频。 |
| `assets` | object[] | 否 | 全部参考素材，见 3.2。 |
| `callback_url` | string | 否 | 任务回调地址，仅部分模型支持，见 3.3。 |
| `seed` | integer | 否 | 随机种子，仅部分模型支持，见 3.3。 |
| `watermark` | boolean | 否 | 是否添加水印，仅部分模型支持，见 3.3。 |
| `options` | object | 否 | 模型专属参数，见 3.3。 |
| `metadata` | object | 否 | 你自己的关联信息，仅部分模型支持，见 3.3。 |

不在上表中的字段会被拒绝并返回 `400`，错误信息会指出字段名。例如带上 `aspect_ratio` 或 `images` 会得到 `aspect_ratio: unknown field`。

### 3.2 assets 素材

所有参考素材（图片、视频、音频、首帧、尾帧）统一放在 `assets[]` 里，每个元素包含：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `type` | 是 | `image`、`video` 或 `audio`。 |
| `url` | 是 | 素材地址，公网可访问的 HTTP(S) 地址。 |
| `role` | 否 | `first_frame`（首帧）、`last_frame`（尾帧）或 `reference`（参考，默认值）。只有 `image` 可以作为首帧或尾帧，且首帧、尾帧各最多一张。 |
| `name` | 否 | 素材名，同一请求内不能重复。不传时自动命名为类型加序号，例如 `image1`、`image2`、`audio1`，已被你使用的名字会被跳过。 |

### 3.3 各模型支持情况

模型能力不同，部分字段只对一部分模型有效。模型不支持的字段或用法会返回 `400`，不会被悄悄忽略。

| 字段 | Seedance（doubao 系列） | 名称带中文的 Seedance 型号 | MiniMax-H3 | Seedance2.5-480全参 | `seedance-933` 等三个型号 |
| --- | --- | --- | --- | --- | --- |
| `prompt` | 与 `assets` 至少提供一项 | 必填 | 与 `assets` 至少提供一项 | 必填 | 与 `assets` 至少提供一项 |
| `generate_audio` | 支持 | 不支持 | 支持 | 支持 | 支持 |
| `seed` | 支持 | 不支持 | 不支持 | 不支持 | 支持 |
| `watermark` | 支持 | 不支持 | 支持 | 支持 | 不支持 |
| `callback_url` | 支持 | 不支持 | 支持 | 不支持 | 不支持 |
| `callback_token` | 不支持 | 不支持 | 支持，需同时传 `callback_url` | 不支持 | 不支持 |
| `metadata` | 不支持 | 支持 | 不支持 | 不支持 | 不支持 |
| 首帧、尾帧（`role`） | 支持 | 支持，但按普通参考图提交 | 支持，不能与参考素材混用 | 支持，尾帧必须同时有首帧，不能与参考素材混用 | 不支持 |
| 参考图片 | 支持 | 支持 | 最多 9 张 | 最多 30 张 | 支持，数量以所选型号为准 |
| 参考视频 | 支持 | 支持 | 最多 3 个 | 最多 10 个 | 支持，数量以所选型号为准 |
| 参考音频 | 支持 | 支持 | 最多 3 个，需同时有参考图片或参考视频 | 最多 10 个 | 支持，数量以所选型号为准 |
| `assets[].name` | 不参与生成 | 提示词中用 `@名称` 引用 | 不参与生成 | 不参与生成 | 不参与生成 |
| `options` | `camera_fixed`、`execution_expires_after`、`seedance_tools`、`omni_reference_task_type` | `native_face` | `use_context_ir`、`skill_id` | 无 | 无 |

表中“不参与生成”的模型没有素材名的概念：`name` 只用来检查同一请求内不重名，提示词里的 `@名称` 会作为普通文字提交，不会按名称绑定素材。

各模型的补充说明：

- **`seedance-2.0-930`、`seedance-2.0-903`**：`model` 必须指定，`prompt` 与 `assets` 至少提供一项；参考素材按类型分别放进图片、视频、音频三组，每组保持你传入的顺序，`assets[].name` 不参与生成；首帧和尾帧必须同时提供，且不能与参考素材混用；不支持 `generate_audio`、`seed`、`watermark`、`callback_url`、`callback_token`、`metadata` 和 `options`。时长范围和参考素材的数量上限由所选型号决定，不符合时请求会被拒绝并返回错误信息。
- **`seedance-933`、`seedance2.0-933-C`、`seedance2.5-V`**：`model` 和 `duration` 都必须指定；参考素材按类型分别放进图片、视频、音频三组，每组保持你传入的顺序，所以提示词里请用“第一张图片”“第二张图片”这样的顺序来指代素材；`assets[].name` 不参与生成，提示词里的 `@名称` 会作为普通文字提交；不支持首帧、尾帧，`role` 写 `first_frame` 或 `last_frame` 会返回 `400`；各型号可用的参考素材类型、数量、时长和画幅以所选型号为准，不符合时请求会被拒绝并返回错误信息。
- **按型号限制的 Seedance 型号**：`model`、`prompt` 和 `duration` 都必须指定（传了参考素材也必须提供 `prompt`）；时长范围、可选分辨率和参考图片、视频、音频的数量上限由所选型号决定，可以从模型列表的 `capabilities` 读取，不符合时返回 `400`；不支持首帧、尾帧（`role` 写 `first_frame` 或 `last_frame` 返回 `400`），也不支持 `generate_audio`、`seed`、`watermark`、`callback_url`、`callback_token`、`metadata` 和 `options`；素材地址必须是公网 HTTPS 地址；`assets[].name` 不参与生成。
- **Seedance2.5-480全参**：`model` 必须指定为 `Seedance2.5-480全参`；`resolution` 只能是 `480p`，省略时也是 `480p`；`duration` 为 4 到 30 秒，省略时为 4 秒；参考图片最多 30 张、参考视频和参考音频各最多 10 个；素材地址必须是公网可访问的 HTTP(S) 地址。统一请求格式只接受 JSON，暂不支持上传首帧图片文件（multipart）。
- **名称带中文的 Seedance 型号**：首帧、尾帧可以照常传，但这个型号的素材没有首尾帧标记，图片会按传入顺序作为普通参考图提交，不保证被当作首帧或尾帧。需要确定的首尾帧效果时，请选用上表中明确支持首尾帧的模型。
- **MiniMax-H3**：`resolution` 写小写，可选 `480p`、`768p`、`1080p`、`2k`、`4k`，其他值会返回 `400`；`duration` 为 4 到 30 秒；`generate_audio` 为 `false` 表示关闭音频；`ratio` 可选 `adaptive`、`21:9`、`16:9`、`4:3`、`1:1`、`3:4`、`9:16`；`options.skill_id` 需要同时设置 `options.use_context_ir` 为 `true`。
- **`wan3.0-video-c3`**：`model`、`prompt` 和 `duration` 都必须指定；`duration` 为 2 到 30 秒；`resolution` 可选 `480p`、`720p`、`1080p`，省略时为 `1080p`；`ratio` 可选 `adaptive`、`21:9`、`16:9`、`4:3`、`1:1`、`3:4`、`9:16`；`seed` 可传；参考图片最多 10 张、参考视频和参考音频各最多 5 个；首帧、尾帧可以照常传。不支持 `generate_audio`、`watermark`、`callback_url`、`callback_token`、`metadata` 和 `options`，传了会返回 `400`。`assets[].name` 不参与生成。

统一请求格式对暂不支持的字段一律报错，而不是悄悄忽略，这样你可以确认每个字段都确实生效。

### 3.4 请求示例

参考素材，并在提示词里用 `@名称` 引用（名称带中文的 Seedance 型号）：

```bash
curl -X POST https://YOUR_SITE/v2/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: video-v2-demo-001" \
  -d '{
    "model": "Seedance 2.5-全参720P",
    "prompt": "@角色.png 在 @场景.png 中自然向前走并回头微笑，配音参考 @配音.wav",
    "duration": 5,
    "resolution": "720p",
    "ratio": "9:16",
    "assets": [
      { "type": "image", "name": "角色.png", "url": "https://example.com/character.png" },
      { "type": "image", "name": "场景.png", "url": "https://example.com/scene.png" },
      { "type": "audio", "name": "配音.wav", "url": "https://example.com/voice.wav" }
    ]
  }'
```

首尾帧，并关闭音频（Seedance（doubao 系列））：

```bash
curl -X POST https://YOUR_SITE/v2/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "人物缓慢转头，背景树叶随风摆动",
    "duration": 5,
    "resolution": "720p",
    "ratio": "16:9",
    "generate_audio": false,
    "assets": [
      { "type": "image", "role": "first_frame", "url": "https://example.com/start.png" },
      { "type": "image", "role": "last_frame", "url": "https://example.com/end.png" }
    ]
  }'
```

参考图并设置回调（MiniMax-H3）：

```bash
curl -X POST https://YOUR_SITE/v2/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "MiniMax-H3",
    "prompt": "晨光照亮湖面，镜头缓慢向前推进",
    "duration": 10,
    "resolution": "1080p",
    "ratio": "16:9",
    "generate_audio": false,
    "callback_url": "https://example.com/callback",
    "assets": [
      { "type": "image", "url": "https://example.com/reference.png" }
    ]
  }'
```

### 3.5 创建响应

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

### 3.6 写法要点

- 所有素材放在 `assets[]`，不使用 `images`、`videos`、`audios`、`image`、`image_url`、`start_frame_url`、`end_frame_url`。
- 时长、分辨率、画幅、音频开关都写在顶层，字段名固定为 `duration`、`resolution`、`ratio`、`generate_audio`。
- `duration` 必须是整数。
- 请求里出现未知字段、模型不支持的字段或用法，会直接返回 `400`。

## 4. 查询模型能力

`GET /v1/models` 返回的每个视频模型都带有 `capabilities` 字段，说明这个模型接受哪些请求写法和参数范围，方便你在发请求之前自行校验：

```json
{
  "object": "list",
  "data": [
    {
      "id": "wan3.0-video-c3",
      "type": "model",
      "display_name": "wan3.0-video-c3",
      "created_at": "2024-01-01T00:00:00Z",
      "capabilities": {
        "protocols": ["v1", "v2"],
        "duration": { "min": 2, "max": 30 },
        "resolutions": ["480p", "720p", "1080p"],
        "ratios": ["adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"],
        "assets": {
          "first_last_frame": true,
          "reference_images": 10,
          "reference_videos": 5,
          "reference_audios": 5
        },
        "generate_audio": false,
        "callback": false
      }
    }
  ]
}
```

| 字段 | 说明 |
| --- | --- |
| `protocols` | 总是返回。`["v1"]` 表示该模型只能使用原生写法，`["v1", "v2"]` 表示还可以使用本文档的统一请求格式。 |
| `duration` | 时长范围，单位秒，`min` 和 `max` 都包含在内。只允许个别取值时会额外返回 `values`，例如 `[4, 6, 8]`。 |
| `resolutions` | 可选的分辨率，统一用小写，例如 `720p`、`4k`，可以直接填入 `resolution`。 |
| `ratios` | 可选的画幅，可以直接填入 `ratio`。少数模型的画幅只对部分请求生效，见 3.3 的补充说明。 |
| `assets` | `first_last_frame` 表示是否支持首尾帧；`reference_images`、`reference_videos`、`reference_audios` 是各类参考素材的数量上限，`0` 表示不支持，没有返回某一项表示没有可确认的固定上限。 |
| `generate_audio` | 是否有音频开关。 |
| `callback` | 是否支持 `callback_url`。 |

某个字段没有返回，表示这一项暂时无法确认。只有 `protocols`、没有其他字段的模型也是这种情况。

## 5. 完整示例（Python）

```python
import time
import requests

base_url = "https://YOUR_SITE"
headers = {
    "Authorization": "Bearer YOUR_API_KEY",
    "Content-Type": "application/json",
}

# 1. 创建任务（统一请求格式）
create = requests.post(
    f"{base_url}/v2/videos/generations",
    headers={**headers, "Idempotency-Key": "video-v2-demo-001"},
    json={
        "model": "doubao-seedance-2-0-260128",
        "prompt": "雨后城市街道，镜头缓慢推进",
        "duration": 5,
        "resolution": "720p",
        "ratio": "16:9",
    },
    timeout=30,
)
create.raise_for_status()
task_id = create.json()["id"]

# 2. 轮询任务状态
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

## 6. 查询任务状态

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

任务通常会经历 `submitted`、`processing`、`succeeded` 或 `failed` 等状态。请间隔数秒轮询，直到任务成功或失败。客户端不需要保持创建请求的长连接；创建接口返回任务 ID 后，应改为轮询查询接口。不要仅凭固定等待时间（例如 20 分钟）判定任务失败，应以查询接口返回的 `succeeded` 或 `failed` 为准；业务侧如需设置最大等待时间，建议按自身体验要求配置，并在超时后继续保留任务 ID 以便稍后查询。

| 状态 | 说明 |
| --- | --- |
| `submitted` | 任务已提交。 |
| `processing` | 任务排队中或生成中。 |
| `succeeded` | 任务成功完成。 |
| `failed` | 任务失败、取消、超时或被上游拒绝。 |

响应中可能包含额外上游字段，请只依赖本文档列出的稳定字段。

## 7. 下载或播放视频

任务成功后返回的 `video_url` 是本站或上游媒体代理地址，可能不会以 `.mp4` 结尾。客户端应根据 HTTP `Content-Type` 或播放器的媒体探测能力处理视频，不要通过文件扩展名判断格式。

部分上游返回的是带签名参数的临时 URL，签名中可能包含 `+`、`/`、`=` 等字符，请注意：

- 保留 URL 的完整内容。如果把视频 URL 放进代理接口的查询参数，必须对完整 URL 做一次 URL 编码（例如 JavaScript 使用 `encodeURIComponent(videoURL)`），不要直接拼接 `?url=`。
- 服务端只解码外层参数一次，不要对已经解码的完整 URL 再次编码。
- 更推荐用 POST JSON 传递 `{ "url": "..." }`，避免嵌套查询参数解析时把 `+` 误解为空格。
- 该 URL 有有效期，过期后请重新查询任务状态获取新的 `video_url`，不要缓存旧的签名 URL。任务完成后请尽快下载并转存。

## 8. 计费与退款

视频生成按本站公布的价格计费，价格以积分计，具体单价请在控制台「模型与价格」页面查看。切换平台分组时，请核对对应平台的模型、参数和价格。

当前规则：

- 创建任务时按预计用量预占余额；任务成功后按实际生成结果结算。
- 任务失败、取消或超时会释放预占金额，失败请求不产生视频费用。
- 任务成功但上游没有返回实际输出时长时，按提交时的输出时长结算。
- 不同模型按输出视频秒数计费（按秒）或按次计费（按次），采用哪种方式以及各分辨率档位的单价，以「模型与价格」页面为准。
- MiniMax-H3 请求 `1080p`、`2k` 或 `4k` 后若实际降级为 `768p`，按 `768p` 的价格结算。创建任务时会锁定所选档位的价格，之后的价格调整不影响进行中的任务；无需由你传入价格。
- 若所选模型或分辨率档位暂未开通价格，创建请求会被拒绝，请联系本站。
- 实际扣费以任务创建时的报价和使用记录为准。

## 9. 常见错误

### 9.1 HTTP 状态

| HTTP 状态 | 场景 | 处理方式 |
| --- | --- | --- |
| `400` | 请求参数无效、没有有效输入、时长或分辨率不支持 | 检查请求字段，`prompt` 与 `assets` 至少提供一项（部分模型必须提供 `prompt`）。 |
| `401` | API Key 缺失或无效 | 检查 `Authorization`。 |
| `402` | 余额或额度不足 | 充值或调整生成参数。 |
| `404` | 当前 API Key 分组不支持视频接口，或任务不存在 | 检查 API Key 分组和任务 ID。 |
| `409` | `Idempotency-Key` 被不同请求复用 | 使用新的唯一 key。 |
| `502` | 上游服务调用失败 | 稍后重试或调整模型能力范围内的参数。 |

### 9.2 参数错误（400）

请求不合法时返回 HTTP `400`，错误类型为 `invalid_request_error`，错误信息的格式是“字段位置: 原因”，字段位置可以精确到 `assets[2].role` 这样的数组元素。常见错误：

| 错误信息示例 | 含义 |
| --- | --- |
| `aspect_ratio: unknown field; the v2 video API does not accept it` | 使用了统一请求格式不支持的字段。 |
| `duration: must be an integer` | 时长不是整数。 |
| `assets[1].type: must be image, video, or audio` | 素材类型不合法。 |
| `assets[1].url: is required` | 素材缺少地址。 |
| `assets[1].name: "a.png" is duplicated` | 素材名重复。 |
| `assets[0].role: "first_frame" is only valid for image assets` | 只有图片可以作为首帧或尾帧。 |
| `assets[1].role: at most one first_frame image is allowed` | 首帧最多一张。 |
| `generate_audio: is not supported by this model` | 该模型没有音频开关。 |
| `assets[0].role: first_frame is not supported by this model` | 该模型不支持首帧、尾帧，请把图片作为普通参考图传入，或选用支持首尾帧的模型。 |
| `resolution: must be one of 480p, 768p, 1080p, 2k, 4k for this model` | 该模型不支持这个分辨率。 |
| `assets: first_frame and last_frame must be provided together for this model` | 该模型要求首帧和尾帧同时提供。 |
| `assets[1]: too many reference images; this model allows at most 1` | 参考素材数量超过该模型的上限。 |
| `model: is required for this model group` | `seedance-933` 等三个型号必须指定 `model`。 |
| `duration: is required for this model group` | `seedance-933` 等三个型号必须指定 `duration`。 |
| `prompt: prompt or at least one asset is required` | 提示词和素材至少提供一项。 |

分组暂不支持统一请求格式时返回 `404`，错误信息为 `The unified video API (v2) is not supported for this platform yet; use /v1/videos/generations`。遇到这种情况请联系本站确认该模型的接入方式。
