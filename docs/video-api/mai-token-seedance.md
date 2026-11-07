# sdtj-2.5-480p 视频生成

> [← 返回视频生成接口](../PP_VIDEO_GENERATION_API.md)

本页说明模型 `sdtj-2.5-480p` 的本站公开接口。创建任务使用 `POST /v1/videos/generations`，任务状态和视频内容均通过本站地址访问。

## 能力与限制

- 模型固定为 `sdtj-2.5-480p`，输出分辨率固定 480P。
- `duration` 必须是 4-30 的 JSON 整数，省略时默认 4 秒。
- 每个任务生成 1 条视频，不支持 `n` 或 `count`。
- 参考图片最多 30 张、参考视频最多 10 个、参考音频最多 10 个，全部参考素材合计最多 50 个。
- 首帧可以单独使用；尾帧必须和首帧一起使用。首尾帧模式不能与普通参考图片、视频或音频混用。
- `generate_audio` 和 `watermark` 必须是 boolean；显式传 `false` 会被保留。
- 参考 URL 必须是可公开访问的 HTTP(S) 地址。multipart 仅支持上传图片文件。

## 原生 JSON

```bash
curl https://YOUR_SITE/v1/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: sdtj-demo-001" \
  -d '{
    "model": "sdtj-2.5-480p",
    "prompt": "清晨海面，镜头缓慢向前推进",
    "duration": 4,
    "resolution": "480p",
    "ratio": "16:9",
    "generate_audio": true,
    "watermark": false
  }'
```

可选参考字段如下：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `referenceImages` | string[] | 普通参考图片 URL，最多 30 张。 |
| `referenceVideos` | string[] | 参考视频 URL，最多 10 个。 |
| `referenceAudios` | string[] | 参考音频 URL，最多 10 个。 |
| `first_image` | string | 首帧图片 URL。 |
| `last_image` | string | 尾帧图片 URL，必须同时提供 `first_image`。 |

字段也兼容 `images`、`videos`、`audios`、`first_frame_url`、`last_frame_url` 等对应别名。不要在同一请求中通过多个别名传入不同的首帧或尾帧。

## video-v1 JSON

`contract_version` 设为 `video-v1` 时，可用统一的 `references` 数组：

```json
{
  "contract_version": "video-v1",
  "model": "sdtj-2.5-480p",
  "prompt": "保持参考主体，镜头平稳推进",
  "duration": 8,
  "resolution": "480p",
  "ratio": "9:16",
  "references": [
    {
      "type": "image",
      "role": "reference_image",
      "source": "https://example.com/reference.jpg"
    }
  ],
  "options": {
    "generate_audio": false
  }
}
```

`type` 可为 `image`、`video` 或 `audio`；对应普通参考角色为 `reference_image`、`reference_video`、`reference_audio`。图片还可使用 `first_frame` 和 `last_frame`。

## 首帧 multipart

使用 `multipart/form-data` 时，文本字段与 JSON 请求含义一致。文件字段只接受 `referenceImages`、`first_image` 和 `last_image`：

```bash
curl https://YOUR_SITE/v1/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Idempotency-Key: sdtj-first-frame-001" \
  -F "model=sdtj-2.5-480p" \
  -F "prompt=保持首帧主体和构图，镜头缓慢推进" \
  -F "duration=4" \
  -F "resolution=480p" \
  -F "ratio=adaptive" \
  -F "generate_audio=false" \
  -F "first_image=@first.jpg;type=image/jpeg"
```

客户端应自行生成 multipart boundary，不要手工设置 `Content-Type` 的 boundary。视频和音频参考只支持 URL，不支持文件上传。

## 查询任务

查询接口为 `GET /v1/videos/{task_id}`。创建成功后保存响应中的 `id`，使用原 API Key 轮询：

```bash
curl https://YOUR_SITE/v1/videos/{task_id} \
  -H "Authorization: Bearer YOUR_API_KEY"
```

状态含义：

| `status` | 说明 |
| --- | --- |
| `queued` / `in_progress` | 任务仍在处理，继续轮询。 |
| `completed` | 生成和本站计费均已完成，可以读取结果。 |
| `failed` | 任务失败，读取错误信息并停止轮询。 |

生成已完成但计费信息尚未就绪，或本地费用仍待结算时，响应不会包含任何视频地址：

```json
{
  "id": "TASK_ID",
  "task_id": "TASK_ID",
  "object": "video",
  "model": "sdtj-2.5-480p",
  "status": "in_progress",
  "billing_pending": true
}
```

此时继续查询原任务，不要重新创建。结算成功后，同一任务会变为 `completed`。

## 下载视频

任务为 `completed` 后，响应中的 `result.url` 和 `video_url` 指向本站认证内容地址。也可直接携带创建任务时的原 API Key 下载：

```bash
curl -L https://YOUR_SITE/v1/videos/{task_id}/content \
  -H "Authorization: Bearer YOUR_API_KEY" \
  --output result.mp4
```

内容接口支持 `GET`、`HEAD` 和 Range 请求。任务未结算、API Key 不属于任务创建者或平台不匹配时，不会返回视频内容。

## 计费说明

创建任务时按请求的输出秒数预扣。完成后只使用响应 `usage.billing_seconds` 的整数值作为最终计费秒数；该值已包含需要计费的参考视频时长和输出时长。本站不会再次相加或取整，也不会把 `usage.output_video_duration` 或 `usage.reference_video_duration` 当作最终计费总秒数。

若 `billing_seconds` 缺失、无效或补扣未完成，任务保持 `billing_pending`，已生成的视频不会公开，后续结算重试也不会重复生成。

## 请求校验

以下请求会在创建前被拒绝：

- 模型不是 `sdtj-2.5-480p`，或分辨率不是精确的 `480p`。
- 时长小于 4、大于 30、为小数或字符串。
- 参考素材超过各类型或总数上限。
- 单独提供尾帧，或把首尾帧与普通参考素材混用。
- URL 不是公开 HTTP(S) 地址，或把视频/音频作为 multipart 文件上传。
- 使用 `seed`、`draft`、回调、`return_last_frame`、多输出等未开放字段。
