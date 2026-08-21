# 站点视频生成接口

本文档是本站对下游客户提供的唯一视频生成接口文档。下游只需要对接本站接口，不需要关心本站后台使用 Jimeng、Kling、Happy Horse、Seedance 或其他上游服务商。

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

## 查询可用模型

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

## 创建视频任务

推荐接口：

```http
POST /v1/videos/generations
```

兼容旧接口：

```http
POST /v1/video/generations
```

请求提交后立即返回任务 ID。视频任务固定异步执行，随后请按“查询任务状态”章节轮询结果。

### 请求字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 否 | 对外视频模型名，建议从 `/v1/models` 获取。未传时按该 API Key 所属平台和账号模型映射自动选择。 |
| `prompt` | string | 条件必填 | 视频描述。`prompt`、参考图字段或首尾帧字段至少提供一项。 |
| `duration` | integer/number/string | 否 | 视频时长，单位秒。建议传 `5`、`10` 或 `15`；未传默认 `5`。 |
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

### 图片输入规则

Seedance 请求必须在下面两种模式中二选一：

- **参考图模式**：使用 `images`（推荐）或兼容字段 `image`、`image_url` 之一。三个字段不能混传。`images` 中的多张图应是同一角色的不同角度或细节补充；图片顺序不代表人物身份或画风的优先级，也不保证锁定人物身份。
- **首尾帧模式**：使用 `start_frame_url` 和可选的 `end_frame_url` 控制视频首尾画面。该模式不能同时传任何参考图字段。

这项限制用于避免上游在“人物参考”和“首帧/尾帧控制”之间自行选择主输入，从而造成角色身份或 3D/真人画风漂移。若目标是保持人物一致性，请只传同一角色的参考图，并在 `prompt` 中明确要求保持角色身份、材质和画风。

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

## 查询任务状态

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

## 下载或播放视频

任务成功后返回的 `video_url` 是本站或上游媒体代理地址，可能不会以 `.mp4` 结尾。客户端应根据 HTTP `Content-Type` 或播放器的媒体探测能力处理视频，不要通过文件扩展名判断格式。

## 计费与退款

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

- K-Ling 的价格档位由请求中的 `mode` 决定；音频参数不改变本站的站内单价。
- 上述基础费用还会叠加本站分组倍率、视频独立倍率（如果启用）和账号倍率。
- `videos`、`audios`、`input_video_duration`、输出宽高和帧率等字段会按上游能力透传，但不会改变本站的按秒计费结果。

## Python 轮询示例

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

## 常见错误

| HTTP 状态 | 场景 | 处理方式 |
| --- | --- | --- |
| `400` | 请求参数无效、没有有效输入、时长或分辨率不支持 | 检查请求字段，至少传入 `prompt`、`image` 或 `images` 之一。 |
| `401` | API Key 缺失或无效 | 检查 `Authorization`。 |
| `402` | 余额或额度不足 | 充值或调整生成参数。 |
| `404` | 当前 API Key 分组不支持视频接口，或任务不存在 | 检查 API Key 分组和任务 ID。 |
| `409` | `Idempotency-Key` 被不同请求复用 | 使用新的唯一 key。 |
| `502` | 上游服务调用失败 | 稍后重试或调整模型能力范围内的参数。 |
