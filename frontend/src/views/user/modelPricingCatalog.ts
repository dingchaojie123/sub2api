export type ModelCategoryId = 'video' | 'image' | 'text'

/** 单个价格档位，amount / suffix 均为可直接展示的文本（例如 "19" + "积分 / 次"）。 */
export interface ModelPriceTier {
  label: string
  amount: string
  suffix: string
}

export interface ModelSpec {
  label: string
  value: string
}

export interface ModelSquareItem {
  id: string
  name: string
  category: ModelCategoryId
  /** 渠道，用作筛选项和卡片角标，取值见 channelOrder */
  channel: string
  /** 计费方式，取值见 billingOrder */
  billing: string
  /** 传给 ModelIcon 的模型名，用于匹配品牌图标 */
  iconModel: string
  description: string
  prices: ModelPriceTier[]
  specs: ModelSpec[]
}

export const categoryLabels: Record<ModelCategoryId, string> = {
  video: '视频',
  image: '图片',
  text: '文本',
}

export const channelOrder = ['官方', '官key', '官转', '特价', '常规']

export const billingOrder = ['按次', '按秒', '按分辨率', '按张', '按 Token']

const credit = (label: string, amount: string, unit: '次' | '秒' | '张'): ModelPriceTier => ({
  label,
  amount,
  suffix: `积分 / ${unit}`,
})

const spec = (label: string, value: string): ModelSpec => ({ label, value })

const refCount = (count: number, unit: string) => (count > 0 ? `最多 ${count} ${unit}` : '不支持')

/** 参考素材规格：图片 / 视频 / 音频各自上限，以及可选的素材合计上限。 */
const refSpecs = (images: number, videos: number, audios: number, total?: number): ModelSpec[] => [
  spec('参考图片', refCount(images, '张')),
  spec('参考视频', refCount(videos, '个')),
  spec('参考音频', refCount(audios, '个')),
  ...(total ? [spec('素材合计上限', `最多 ${total} 份`)] : []),
]

const RATIOS_ALL = '16:9 / 9:16 / 1:1 / 4:3 / 3:4 / 21:9'
const MEDIA_LIMIT = '单素材 ≤64 MiB；素材时长 ≤30 秒'

const officialVideoModels: ModelSquareItem[] = [
  {
    id: 'official-seedance-2.5',
    name: 'Seedance 2.5',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'doubao',
    description:
      '支持文生视频、首尾帧；可用真人照片作参考；支持编辑和延长参考视频；默认生成音频，可关闭；首尾帧不可与其他参考素材混用。',
    prices: [credit('480P', '1.8', '秒'), credit('720P', '4', '秒'), credit('1080P', '10', '秒')],
    specs: [
      spec('时长', '4–30 秒（不传由模型自动决定）'),
      spec('分辨率', '480P / 720P / 1080P，默认 720P'),
      spec('画面比例', RATIOS_ALL),
      ...refSpecs(30, 10, 10, 50),
    ],
  },
  {
    id: 'official-seedance-2.0',
    name: 'Seedance 2.0',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'doubao',
    description:
      '支持文生视频、首尾帧；可用真人照片作参考；默认生成音频，可关闭；首尾帧不可与其他参考素材混用。',
    prices: [credit('480P', '1.3', '秒'), credit('720P', '2.6', '秒'), credit('1080P', '6.5', '秒')],
    specs: [
      spec('时长', '4–15 秒，默认 5 秒'),
      spec('分辨率', '480P / 720P / 1080P / 4K，默认 720P'),
      spec('画面比例', RATIOS_ALL),
      ...refSpecs(9, 3, 3, 15),
    ],
  },
  {
    id: 'official-seedance-2.0-mini',
    name: 'Seedance 2.0 Mini',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'doubao',
    description:
      '支持文生视频和首尾帧，也支持参考生成但参考能力有限，不适合做全参考使用；价格实惠，适合低成本草稿、测试和批量生成；默认生成音频，可关闭。',
    prices: [credit('480P', '0.3', '秒'), credit('720P', '0.63', '秒')],
    specs: [
      spec('时长', '4–15 秒，默认 5 秒'),
      spec('分辨率', '480P / 720P，默认 720P'),
      spec('画面比例', RATIOS_ALL),
      ...refSpecs(9, 3, 3, 15),
    ],
  },
  {
    id: 'official-seedance-2.0-fast',
    name: 'Seedance 2.0 Fast',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'doubao',
    description:
      '高性价比，适合快速预览和批量创作；支持文生视频、首尾帧，可用真人照片作参考；首尾帧不可与其他参考素材混用；默认生成音频，可关闭。',
    prices: [credit('480P', '0.6', '秒'), credit('720P', '1.3', '秒')],
    specs: [
      spec('时长', '4–15 秒，默认 5 秒'),
      spec('分辨率', '480P / 720P，默认 720P'),
      spec('画面比例', RATIOS_ALL),
      ...refSpecs(9, 3, 3, 15),
    ],
  },
  {
    id: 'official-gemini-omni-flash',
    name: 'Gemini Omni Flash',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'gemini',
    description:
      'Google 预览版；支持文生视频、首帧或首尾帧图生视频；不支持视频编辑、视频延长和参考音频；图片支持 PNG/JPEG/WebP 链接、Base64 或文件上传。',
    prices: [credit('720P', '0.5', '秒')],
    specs: [
      spec('时长', '3–10 秒，默认 3 秒'),
      spec('分辨率', '720P（固定）'),
      spec('画面比例', '16:9 / 9:16'),
      ...refSpecs(10, 1, 0, 11),
    ],
  },
  {
    id: 'official-gemini-omni-flash-1.1',
    name: 'Gemini Omni Flash 1.1',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'gemini',
    description:
      '支持视频编辑与视频延长（每次新增约 10 秒，按新增秒数计费，输入视频 1–30 秒）；图片支持链接、Base64 或文件上传；不支持参考音频。',
    prices: [credit('720P', '0.5', '秒'), credit('1080P', '0.6', '秒'), credit('4K', '0.8', '秒')],
    specs: [
      spec('时长', '3–10 秒，默认 3 秒'),
      spec('分辨率', '720P / 1080P / 4K'),
      spec('画面比例', '16:9 / 9:16'),
      ...refSpecs(10, 3, 0, 13),
      spec('单文件 / 时长限制', '视频需 HTTP(S) 链接（MP4/MOV/WebM，≤64MiB）'),
    ],
  },
  {
    id: 'official-grok-imagine-video',
    name: 'Grok Imagine Video',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'grok',
    description: '纯文生视频或最多 1 张首帧图生视频；生成结果默认带音轨。',
    prices: [credit('480P', '0.5', '秒'), credit('720P', '0.7', '秒')],
    specs: [
      spec('时长', '1–15 秒，默认 8 秒'),
      spec('分辨率', '480P / 720P'),
      spec('画面比例', '16:9 / 9:16 / 1:1 / 4:3 / 3:4 / 3:2 / 2:3'),
      ...refSpecs(1, 0, 0, 1),
    ],
  },
  {
    id: 'official-grok-imagine-video-1.5',
    name: 'Grok Imagine Video 1.5',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'grok',
    description: '纯文生视频或最多 1 张首帧图生视频；生成结果默认带音轨；不支持参考视频或参考音频。',
    prices: [credit('480P', '0.2', '秒'), credit('720P', '0.4', '秒'), credit('1080P', '0.5', '秒')],
    specs: [
      spec('时长', '1–15 秒，默认 8 秒'),
      spec('分辨率', '480P / 720P / 1080P'),
      spec('画面比例', '16:9 / 9:16 / 1:1 / 4:3 / 3:4 / 3:2 / 2:3'),
      ...refSpecs(1, 0, 0, 1),
    ],
  },
  {
    id: 'official-kling-3.0',
    name: 'Kling 3.0',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'kling',
    description:
      '默认无声视频，generateAudio: true 生成有声；首尾帧模式下画面比例跟随首帧，且不能与参考图同时使用；不支持参考视频和参考音频。',
    prices: [credit('720P', '2', '秒'), credit('1080P', '2.8', '秒')],
    specs: [
      spec('时长', '3–15 秒，默认 5 秒'),
      spec('分辨率', '720P / 1080P，默认 720P'),
      spec('画面比例', '16:9 / 1:1 / 9:16，默认 16:9'),
      ...refSpecs(6, 0, 0, 6),
      spec('计费说明', '按分辨率计费（有声 / 无声分价）'),
    ],
  },
  {
    id: 'official-kling-4.0-omni',
    name: 'Kling 4.0 Omni',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'kling',
    description:
      '固定同时生成画面和音频，不可关闭；首尾帧可与参考图同时使用，比例跟随首帧；支持 negativePrompt（≤2000 字）、seed、enhancePrompt；不支持参考音频。',
    prices: [credit('720P', '2.2', '秒'), credit('1080P', '3.4', '秒')],
    specs: [
      spec('时长', '3–30 秒，默认 5 秒'),
      spec('分辨率', '720P / 1080P，默认 720P'),
      spec('画面比例', '21:9 / 16:9 / 1:1 / 9:16，默认 16:9'),
      ...refSpecs(10, 5, 0, 15),
      spec('单文件 / 时长限制', '参考视频总时长 3–30 秒'),
      spec(
        '计费说明',
        '按分辨率计费；参考视频按「参考视频 + 生成视频」总秒数、参考视频档位计费（预扣后多退少补）',
      ),
    ],
  },
  {
    id: 'official-veo-3.1',
    name: 'Veo 3.1',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'gemini',
    description:
      '支持文生视频、首帧或首尾帧图生视频、1–3 张参考图生成、视频延长（每次新增 7 秒，按新增秒数计费）；默认生成原生同步音频，可关闭；图片支持 JPEG/PNG 链接、Base64 或文件上传。',
    prices: [credit('720P', '0.56', '秒'), credit('1080P', '0.7', '秒'), credit('4K', '0.84', '秒')],
    specs: [
      spec('时长', '4 / 6 / 8 秒，默认 8 秒（参考图生成固定 8 秒）'),
      spec('分辨率', '720P / 1080P / 4K'),
      spec('画面比例', '16:9 / 9:16'),
      ...refSpecs(3, 0, 0, 3),
      spec('单文件 / 时长限制', '延长视频需提供 HTTP(S) 链接'),
    ],
  },
  {
    id: 'official-veo-3.1-fast',
    name: 'Veo 3.1 Fast',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'gemini',
    description:
      '侧重速度与成本，适合快速预览、批量素材和创意迭代；能力与 Veo 3.1 一致：文生视频、首帧或首尾帧、1–3 张参考图生成、视频延长（每次新增 7 秒）；默认原生同步音频可关。',
    prices: [credit('720P', '0.4', '秒'), credit('1080P', '0.5', '秒'), credit('4K', '0.7', '秒')],
    specs: [
      spec('时长', '4 / 6 / 8 秒，默认 8 秒（参考图生成固定 8 秒）'),
      spec('分辨率', '720P / 1080P / 4K'),
      spec('画面比例', '16:9 / 9:16'),
      ...refSpecs(3, 0, 0, 3),
      spec('单文件 / 时长限制', '延长视频需提供 HTTP(S) 链接'),
    ],
  },
  {
    id: 'official-wan-3.0-video',
    name: 'Wan 3.0 Video',
    category: 'video',
    channel: '官方',
    billing: '按分辨率',
    iconModel: 'qwen',
    description:
      '回调为官方 URL；支持文/图生视频（首帧、首尾帧）及图、视频、音频参考；图/音频/文档/网页不计费。',
    prices: [credit('480P', '0.8', '秒'), credit('720P', '1.4', '秒'), credit('1080P', '2', '秒')],
    specs: [
      spec('时长', '2–30 秒或 -1（智能时长）'),
      spec('分辨率', '480P / 720P / 1080P'),
      spec(
        '计费说明',
        '输入视频 + 输出视频时长 × 输出分辨率单价；无参考视频只计输出；参考视频/智能时长预扣 30 秒，按实际上游秒数结算',
      ),
    ],
  },
]

const channelVideoModels: ModelSquareItem[] = [
  {
    id: 'limited-seedance-2.5-full',
    name: '限时限量-Seedance 2.5 满参',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description:
      '限时限量特价通道，满参考 30 图 / 10 视频 / 10 音频。每天额度有限，用完为止；素材合计最多 30 份。',
    prices: [credit('720P', '19', '次')],
    specs: [
      spec('时长', '4–30 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', '9:16 / 16:9 / 1:1 / 4:3 / 3:4 / 21:9'),
      ...refSpecs(30, 10, 10, 30),
      spec('提示词上限', '45000 字'),
    ],
  },
  {
    id: 'regular-seedance-2.5-no-video',
    name: 'Seedance 2.5-无视频',
    category: 'video',
    channel: '常规',
    billing: '按秒',
    iconModel: 'doubao',
    description: '常规通道，支持 30 图 / 10 音频参考（不支持参考视频），唯一支持 adaptive 自适应画面比例。',
    prices: [credit('720P', '1.2', '秒')],
    specs: [
      spec('时长', '4–30 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', `${RATIOS_ALL} / adaptive`),
      ...refSpecs(30, 0, 10, 40),
      spec('提示词上限', '20000 字'),
      spec('单文件 / 时长限制', MEDIA_LIMIT),
    ],
  },
  {
    id: 'limited-seedance-2.5-full-backup',
    name: '限时限量-Seedance 2.5 满参-备胎',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: '限时限量满参版备用通道，满参考 30 图 / 10 视频 / 10 音频，画面比例不含 21:9。',
    prices: [credit('720P', '19', '次')],
    specs: [
      spec('时长', '5–30 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', '16:9 / 9:16 / 4:3 / 3:4 / 1:1'),
      ...refSpecs(30, 10, 10, 50),
      spec('提示词上限', '20000 字'),
    ],
  },
  {
    id: 'regular-seedance-2.5-no-ref-video',
    name: 'Seedance 2.5-无参考视频',
    category: 'video',
    channel: '常规',
    billing: '按秒',
    iconModel: 'doubao',
    description: '常规通道，不支持参考视频，原生过人脸；血腥暴力、色情类提示词及版权内容会被拦截。',
    prices: [credit('480P', '1.2', '秒'), credit('720P', '1.6', '秒')],
    specs: [
      spec('时长', '5–30 秒'),
      spec('分辨率', '480P / 720P'),
      spec('画面比例', RATIOS_ALL),
      ...refSpecs(30, 0, 10),
      spec('提示词上限', '20000 字'),
      spec('单文件 / 时长限制', MEDIA_LIMIT),
    ],
  },
  {
    id: 'special-seedance-2.0',
    name: '特价-Seedance 2.0',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: '特价通道，支持 9 图 / 3 视频 / 3 音频参考，画面比例仅支持 9:16 / 16:9 / 1:1 三种。',
    prices: [credit('720P', '6.8', '次')],
    specs: [
      spec('时长', '4–15 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', '9:16 / 16:9 / 1:1'),
      ...refSpecs(9, 3, 3, 15),
      spec('提示词上限', '20000 字'),
      spec('单文件 / 时长限制', '参考视频 + 生成视频合计 ≤25 秒'),
    ],
  },
  {
    id: 'regular-seedance-2.0-no-ref-video',
    name: 'Seedance 2.0-无参考视频',
    category: 'video',
    channel: '常规',
    billing: '按秒',
    iconModel: 'doubao',
    description:
      '常规通道，不支持参考视频，480P / 720P / 1080P 三档分辨率可选，原生过人脸；血腥暴力、色情类提示词及版权内容会被拦截。',
    prices: [credit('480P', '0.9', '秒'), credit('720P', '1.2', '秒'), credit('1080P', '1.8', '秒')],
    specs: [
      spec('时长', '5–15 秒'),
      spec('分辨率', '480P / 720P / 1080P'),
      spec('画面比例', RATIOS_ALL),
      ...refSpecs(9, 0, 3),
      spec('提示词上限', '10000 字'),
      spec('单文件 / 时长限制', MEDIA_LIMIT),
    ],
  },
  {
    id: 'portrait-seedance-2.0',
    name: '竖屏-Seedance 2.0',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: '竖屏特价通道，仅支持 9:16，可纯文本出片；限制儿童、血腥暴力及负能量内容。',
    prices: [credit('720P', '4', '次')],
    specs: [
      spec('时长', '4–15 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', '9:16'),
      ...refSpecs(9, 3, 3, 12),
      spec(
        '单文件 / 时长限制',
        '图片 ≤15 MiB；视频/音频 ≤5 MiB；单段 2–15 秒，音视频各自累计 ≤15 秒；至少 1 图或 1 视频',
      ),
    ],
  },
  {
    id: 'portrait-seedance-2.0-fast',
    name: '竖屏-Seedance 2.0 Fast',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: '竖屏快出片通道，仅支持 9:16，素材合计 15 个。',
    prices: [credit('720P', '3.2', '次')],
    specs: [
      spec('时长', '5–15 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', '9:16'),
      ...refSpecs(9, 3, 3, 15),
      spec(
        '单文件 / 时长限制',
        '图片 ≤15 MiB；视频/音频 ≤5 MiB；单段 2–15 秒，各类累计 ≤15 秒；至少 1 图或 1 视频',
      ),
    ],
  },
  {
    id: 'portrait-seedance-2.0-mini',
    name: '竖屏-Seedance 2.0 Mini',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: '竖屏按次通道，价格最低，仅支持 9:16。',
    prices: [credit('720P', '2.3', '次')],
    specs: [
      spec('时长', '5–15 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', '9:16'),
      ...refSpecs(9, 3, 3, 15),
      spec(
        '单文件 / 时长限制',
        '图片 ≤15 MiB；视频/音频 ≤5 MiB；单段 2–15 秒，各类累计 ≤15 秒；至少 1 图或 1 视频',
      ),
    ],
  },
  {
    id: 'special-seedance-2.0-933',
    name: 'Seedance2.0-933',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: '参考 9 图 / 0 视频 / 3 音频，720P，原生过真人，可生成 4–15 秒。',
    prices: [credit('720P', '6.5', '次')],
    specs: [
      spec('时长', '4–15 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', RATIOS_ALL),
      ...refSpecs(9, 0, 3, 15),
      spec('提示词上限', '15000 字'),
    ],
  },
  {
    id: 'special-seedance-2.0-933-c',
    name: 'Seedance2.0-933-C',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: '参考 9 图 / 3 视频 / 3 音频，720P，过真人，可生成 4–15 秒，按条收费。',
    prices: [credit('720P', '6.5', '次')],
    specs: [
      spec('时长', '4–15 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', RATIOS_ALL),
      ...refSpecs(9, 3, 3, 15),
    ],
  },
  {
    id: 'special-seedance-2.5-v',
    name: 'Seedance2.5-V',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: '参考 30 图 / 10 视频 / 10 音频，720P，原生过真人，支持 5–30 秒，支持全比例。',
    prices: [credit('720P', '19', '次')],
    specs: [
      spec('时长', '5–30 秒'),
      spec('分辨率', '720P'),
      spec('画面比例', RATIOS_ALL),
      ...refSpecs(30, 10, 10, 30),
      spec('提示词上限', '15000 字'),
    ],
  },
]

/** 沿用原价格表（人民币）的视频模型，xlsx 未覆盖的部分。 */
const legacyVideoModels: ModelSquareItem[] = [
  {
    id: 'legacy-grok-imagine-video-key',
    name: 'grok-imagine-video',
    category: 'video',
    channel: '官key',
    billing: '按秒',
    iconModel: 'grok',
    description: 'Grok Imagine 视频生成（官 key 渠道），按生成时长和分辨率计费。',
    prices: [credit('480P', '0.18', '秒'), credit('720P', '0.24', '秒')],
    specs: [spec('分辨率', '480P / 720P')],
  },
  {
    id: 'legacy-kling-3.0-turbo',
    name: 'kling-3.0-turbo',
    category: 'video',
    channel: '官方',
    billing: '按秒',
    iconModel: 'kling',
    description: 'Kling 3.0 Turbo 视频生成，按生成时长计费。',
    prices: [credit('720P', '1.30', '秒')],
    specs: [spec('分辨率', '720P')],
  },
  {
    id: 'legacy-minimax',
    name: 'MiniMax',
    category: 'video',
    channel: '官方',
    billing: '按秒',
    iconModel: 'minimax',
    description: 'MiniMax 视频生成，按生成时长和分辨率计费。',
    prices: [
      credit('480P', '0.30', '秒'),
      credit('768P', '0.40', '秒'),
      credit('1080P', '0.60', '秒'),
      credit('2K', '1.00', '秒'),
      credit('4K', '2.00', '秒'),
    ],
    specs: [spec('分辨率', '480P / 768P / 1080P / 2K / 4K')],
  },
  {
    id: 'legacy-pixverse-v6',
    name: 'Pixverse-V6',
    category: 'video',
    channel: '官方',
    billing: '按秒',
    iconModel: 'pixverse',
    description: 'Pixverse V6 视频生成，按生成时长和分辨率计费。',
    prices: [credit('480P', '0.30', '秒'), credit('720P', '0.48', '秒'), credit('1080P', '0.72', '秒')],
    specs: [spec('分辨率', '480P / 720P / 1080P')],
  },
]

const subsidizedVideoModels: ModelSquareItem[] = [
  {
    id: 'tiktok-seedance-2.0-9-3-0',
    name: 'seedance-2.0 · 9图 / 3视频 / 0音频',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: 'Seedance 2.0 特价模型，参考 9 图 / 3 视频 / 0 音频，按次计费。',
    prices: [credit('720P', '3.36', '次')],
    specs: [spec('分辨率', '720P'), spec('时长', '15 秒'), spec('折合价格', '约 0.22 积分 / 秒')],
  },
  {
    id: 'tiktok-seedance-2.0-9-0-3',
    name: 'seedance-2.0 · 9图 / 0视频 / 3音频',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: 'Seedance 2.0 特价模型，参考 9 图 / 0 视频 / 3 音频，按次计费。',
    prices: [credit('720P', '3.36', '次')],
    specs: [spec('分辨率', '720P'), spec('时长', '15 秒'), spec('折合价格', '约 0.22 积分 / 秒')],
  },
  {
    id: 'tiktok-seedance-2.0-9-0-3-real',
    name: 'seedance-2.0 · 9图 / 0视频 / 3音频 · 原生过真人',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: 'Seedance 2.0 特价模型，参考 9 图 / 0 视频 / 3 音频，原生过真人，按次计费。',
    prices: [credit('720P', '4.70', '次')],
    specs: [spec('分辨率', '720P'), spec('时长', '15 秒'), spec('折合价格', '约 0.31 积分 / 秒')],
  },
  {
    id: 'tiktok-seedance-2.5-face',
    name: 'seedance-2.5 · 满参过人脸（参考视频不能真人）',
    category: 'video',
    channel: '特价',
    billing: '按次',
    iconModel: 'doubao',
    description: 'Seedance 2.5 特价模型，满参过人脸（参考视频不能真人），按次计费。',
    prices: [credit('720P', '14.00', '次'), credit('1080P', '17.00', '次')],
    specs: [
      spec('分辨率', '720P / 1080P'),
      spec('时长', '30 秒'),
      spec('折合价格', '720P 约 0.46 积分 / 秒；1080P 约 0.56 积分 / 秒'),
    ],
  },
  {
    id: 'tiktok-seedance-2.5-full',
    name: 'seedance-2.5 · 满参 30图 / 10视频 / 10音频',
    category: 'video',
    channel: '特价',
    billing: '按秒',
    iconModel: 'doubao',
    description: 'Seedance 2.5 特价模型，满参考 30 图 / 10 视频 / 10 音频，按生成秒数计费。',
    prices: [credit('720P', '0.80', '秒')],
    specs: [spec('分辨率', '720P'), spec('时长', '5–30 秒')],
  },
  {
    id: 'tiktok-seedance-2.0-full',
    name: 'seedance-2.0 · 满参 9图 / 3视频 / 3音频',
    category: 'video',
    channel: '特价',
    billing: '按秒',
    iconModel: 'doubao',
    description: 'Seedance 2.0 特价模型，满参考 9 图 / 3 视频 / 3 音频，按生成秒数计费。',
    prices: [credit('720P', '0.80', '秒')],
    specs: [spec('分辨率', '720P'), spec('时长', '5–15 秒')],
  },
]

const imageModels: ModelSquareItem[] = [
  {
    id: 'image-gpt-image',
    name: 'gpt-image',
    category: 'image',
    channel: '官方',
    billing: '按张',
    iconModel: 'gpt-image',
    description: 'GPT 图片生成，按生成张数和分辨率计费。',
    prices: [credit('1K', '0.19', '张'), credit('2K', '0.49', '张'), credit('4K', '0.81', '张')],
    specs: [spec('分辨率', '1K / 2K / 4K')],
  },
  {
    id: 'image-gemini',
    name: 'Gemini图片',
    category: 'image',
    channel: '官方',
    billing: '按张',
    iconModel: 'gemini',
    description: 'Gemini 图片生成，按生成张数和分辨率计费。',
    prices: [credit('1K', '0.19', '张'), credit('2K', '0.49', '张'), credit('4K', '0.81', '张')],
    specs: [spec('分辨率', '1K / 2K / 4K')],
  },
  {
    id: 'image-midjourney',
    name: 'Midjourney',
    category: 'image',
    channel: '官方',
    billing: '按张',
    iconModel: 'midjourney',
    description: 'Midjourney 图片生成，按生成张数和分辨率计费。',
    prices: [credit('1K', '0.97', '张'), credit('2K', '1.46', '张'), credit('4K', '1.94', '张')],
    specs: [spec('分辨率', '1K / 2K / 4K')],
  },
]

const textModels: ModelSquareItem[] = [
  {
    id: 'text-gpt-official',
    name: 'GPT',
    category: 'text',
    channel: '官方',
    billing: '按 Token',
    iconModel: 'gpt',
    description: 'GPT 文本模型，按实际 Token 消耗量计费。',
    prices: [{ label: '折扣', amount: '7 折', suffix: '' }],
    specs: [spec('计费说明', '按 Token 消耗量计费')],
  },
  {
    id: 'text-claude-code-official',
    name: 'Claude Code',
    category: 'text',
    channel: '官方',
    billing: '按 Token',
    iconModel: 'claude',
    description: 'Claude Code 文本模型，按实际 Token 消耗量计费。',
    prices: [{ label: '折扣', amount: '7 折', suffix: '' }],
    specs: [spec('计费说明', '按 Token 消耗量计费')],
  },
  {
    id: 'text-gpt-transfer',
    name: 'GPT',
    category: 'text',
    channel: '官转',
    billing: '按 Token',
    iconModel: 'gpt',
    description: 'GPT 文本模型（官转渠道），按实际 Token 消耗量计费。',
    prices: [{ label: '折扣', amount: '3 折', suffix: '' }],
    specs: [spec('计费说明', '按 Token 消耗量计费')],
  },
  {
    id: 'text-claude-code-transfer',
    name: 'Claude Code',
    category: 'text',
    channel: '官转',
    billing: '按 Token',
    iconModel: 'claude',
    description: 'Claude Code 文本模型（官转渠道），按实际 Token 消耗量计费。',
    prices: [{ label: '折扣', amount: '3 折', suffix: '' }],
    specs: [spec('计费说明', '按 Token 消耗量计费')],
  },
]

export const modelSquareCatalog: ModelSquareItem[] = [
  ...officialVideoModels,
  ...channelVideoModels,
  ...legacyVideoModels,
  ...subsidizedVideoModels,
  ...imageModels,
  ...textModels,
]
