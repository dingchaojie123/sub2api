export type ModelPricingCategoryId = 'video' | 'subsidized-video' | 'image' | 'text'

export interface ModelPricingColumn {
  key: string
  label: string
  price?: boolean
}

export interface ModelPricingRow {
  model: string
  values: Record<string, string>
  searchTerms?: string
}

export interface ModelPricingCategory {
  id: ModelPricingCategoryId
  title: string
  description: string
  unit: string
  columns: ModelPricingColumn[]
  rows: ModelPricingRow[]
}

const unavailable = '-'

export const modelPricingCatalog: ModelPricingCategory[] = [
  {
    id: 'video',
    title: '视频生成',
    description: '常规视频模型按生成时长计费',
    unit: '元 / 秒',
    columns: [
      { key: '480p', label: '480P', price: true },
      { key: '720p', label: '720P', price: true },
      { key: '1080p', label: '1080P', price: true },
      { key: '2k', label: '2K', price: true },
      { key: '4k', label: '4K', price: true },
    ],
    rows: [
      { model: 'grok-imagine-video【官key】', values: { '480p': '¥0.18', '720p': '¥0.24', '1080p': unavailable, '2k': unavailable, '4k': unavailable } },
      { model: 'gemini-omni-flash【官】', values: { '480p': unavailable, '720p': '¥0.30', '1080p': unavailable, '2k': unavailable, '4k': unavailable } },
      { model: 'grok-imagine-video-1.5【官】', values: { '480p': '¥0.10', '720p': '¥0.24', '1080p': '¥0.30', '2k': unavailable, '4k': unavailable } },
      { model: 'veo-3.1【官】', values: { '480p': unavailable, '720p': '¥0.36', '1080p': '¥0.42', '2k': unavailable, '4k': '¥0.54' } },
      { model: 'veo-3.1-fast【官】', values: { '480p': unavailable, '720p': '¥0.24', '1080p': '¥0.30', '2k': unavailable, '4k': '¥0.42' } },
      { model: 'seedance-2.0-mini【官】', values: { '480p': '¥0.24', '720p': '¥0.48', '1080p': unavailable, '2k': unavailable, '4k': unavailable } },
      { model: 'seedance-2.0-fast【官】', values: { '480p': unavailable, '720p': '¥1.00', '1080p': unavailable, '2k': unavailable, '4k': unavailable } },
      { model: 'seedance-2.0【官】', values: { '480p': '¥0.50', '720p': '¥1.00', '1080p': '¥2.80', '2k': unavailable, '4k': '¥5.80' } },
      { model: 'seedance-2.5【官】', values: { '480p': '¥0.70', '720p': '¥1.30', '1080p': '¥4.50', '2k': unavailable, '4k': unavailable } },
      { model: 'gemini-omni-flash-1.1【官】', values: { '480p': unavailable, '720p': '¥0.30', '1080p': '¥0.36', '2k': unavailable, '4k': '¥0.48' } },
      { model: 'kling-3.0-turbo【官】', values: { '480p': unavailable, '720p': '¥1.30', '1080p': unavailable, '2k': unavailable, '4k': unavailable } },
      { model: 'Wan3.0【官】', values: { '480p': '¥0.50', '720p': '¥1.20', '1080p': '¥2.40', '2k': unavailable, '4k': unavailable } },
      { model: 'MiniMax【官】', values: { '480p': '¥0.10', '720p': '¥0.24', '1080p': '¥0.36', '2k': '¥0.60', '4k': '¥1.20' } },
      { model: 'Pixverse-V6【官】', values: { '480p': '¥0.30', '720p': '¥0.48', '1080p': '¥0.72', '2k': unavailable, '4k': unavailable } },
    ],
  },
  {
    id: 'subsidized-video',
    title: 'TikTok 补贴视频',
    description: 'Seedance 补贴模型包含按次和按秒两种计费方式',
    unit: '人民币',
    columns: [
      { key: 'resolution', label: '分辨率' },
      { key: 'billing', label: '计费方式' },
      { key: 'price', label: '价格', price: true },
      { key: 'duration', label: '时长' },
      { key: 'note', label: '折合价格', price: true },
    ],
    rows: [
      { model: 'seedance-2.0 · 9图 / 3视频 / 0音频', values: { resolution: '720P', billing: '按次', price: '¥3.36 / 次', duration: '15 秒', note: '约 ¥0.22 / 秒' } },
      { model: 'seedance-2.0 · 9图 / 0视频 / 3音频', values: { resolution: '720P', billing: '按次', price: '¥3.36 / 次', duration: '15 秒', note: '约 ¥0.22 / 秒' } },
      { model: 'seedance-2.0 · 9图 / 0视频 / 3音频 · 原生过真人', values: { resolution: '720P', billing: '按次', price: '¥4.70 / 次', duration: '15 秒', note: '约 ¥0.31 / 秒' } },
      { model: 'seedance-2.5 · 满参过人脸（参考视频不能真人）', values: { resolution: '720P', billing: '按次', price: '¥14.00 / 次', duration: '30 秒', note: '约 ¥0.46 / 秒' } },
      { model: 'seedance-2.5 · 满参过人脸（参考视频不能真人）', values: { resolution: '1080P', billing: '按次', price: '¥17.00 / 次', duration: '30 秒', note: '约 ¥0.56 / 秒' } },
      { model: 'seedance-2.5 · 满参 30图 / 10视频 / 10音频', values: { resolution: '720P', billing: '按秒', price: '¥0.80 / 秒', duration: '5-30 秒', note: unavailable } },
      { model: 'seedance-2.0 · 满参 9图 / 3视频 / 3音频', values: { resolution: '720P', billing: '按秒', price: '¥0.80 / 秒', duration: '5-15 秒', note: unavailable } },
    ],
  },
  {
    id: 'image',
    title: '图片生成',
    description: '图片模型按生成张数和分辨率计费',
    unit: '元 / 张',
    columns: [
      { key: '1k', label: '1K', price: true },
      { key: '2k', label: '2K', price: true },
      { key: '4k', label: '4K', price: true },
    ],
    rows: [
      { model: 'gpt-image【官】', values: { '1k': '¥0.12', '2k': '¥0.30', '4k': '¥0.50' } },
      { model: 'Gemini图片【官】', values: { '1k': '¥0.12', '2k': '¥0.30', '4k': '¥0.50' } },
      { model: 'Midjourney【官】', values: { '1k': '¥0.60', '2k': '¥0.90', '4k': '¥1.20' } },
    ],
  },
  {
    id: 'text',
    title: '文本生成',
    description: '文本模型按实际 Token 消耗量计费',
    unit: 'Token 计费',
    columns: [
      { key: 'discount', label: '折扣', price: true },
      { key: 'billing', label: '计费说明' },
    ],
    rows: [
      { model: 'GPT【官】', values: { discount: '7 折', billing: '按 Token 消耗量计费' } },
      { model: 'Claude Code【官】', values: { discount: '7 折', billing: '按 Token 消耗量计费' } },
      { model: 'GPT【官转】', values: { discount: '3 折', billing: '按 Token 消耗量计费' } },
      { model: 'Claude Code【官转】', values: { discount: '3 折', billing: '按 Token 消耗量计费' } },
    ],
  },
]
