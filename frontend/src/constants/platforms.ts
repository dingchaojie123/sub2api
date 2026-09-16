export const PROVIDER_PLATFORMS = ['doubao', 'qwen', 'kimi', 'deepseek'] as const

export type ProviderPlatform = (typeof PROVIDER_PLATFORMS)[number]

export const VIDEO_ACCOUNT_PLATFORMS = [
  'kling',
  'happyhourse',
  'seedance',
  'bytedance',
  'wan3',
  'minimax-h3',
  'pixverse-v6',
  'grok-imagine-video',
  'kuaishou'
] as const

export type VideoAccountPlatform = (typeof VIDEO_ACCOUNT_PLATFORMS)[number]

export const AUDIO_ACCOUNT_PLATFORMS = ['minimax-speech', 'qwen-tts'] as const

export type AudioAccountPlatform = (typeof AUDIO_ACCOUNT_PLATFORMS)[number]

export const CHANNEL_PRICING_PLATFORMS = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  'jimeng',
  'doubao',
  'qwen',
  'kimi',
  'deepseek',
  'kling',
  'happyhourse',
  'seedance',
  'bytedance',
  'wan3',
  'minimax-h3',
  'minimax-speech',
  'qwen-tts',
  'pixverse-v6',
  'grok-imagine-video',
  'kuaishou'
] as const

export interface ProviderPlatformMetadata {
  id: ProviderPlatform
  label: string
  defaultBaseUrl: string
  baseUrlPlaceholder: string
  apiKeyPlaceholder: string
  accountType: 'apikey'
  modelPlatform: string
}

export interface VideoAccountPlatformMetadata {
  id: VideoAccountPlatform
  label: string
  defaultBaseUrl: string
  baseUrlPlaceholder: string
  apiKeyPlaceholder: string
  accountType: 'apikey'
  authScheme: 'bearer'
}

export interface AudioAccountPlatformMetadata {
  id: AudioAccountPlatform
  label: string
  defaultBaseUrl: string
  baseUrlPlaceholder: string
  apiKeyPlaceholder: string
  accountType: 'apikey'
  authScheme: 'bearer'
}

export const PROVIDER_PLATFORM_METADATA: Record<ProviderPlatform, ProviderPlatformMetadata> = {
  doubao: {
    id: 'doubao',
    label: '豆包',
    defaultBaseUrl: 'https://ark.cn-beijing.volces.com/api/v3',
    baseUrlPlaceholder: 'https://ark.cn-beijing.volces.com/api/v3',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    modelPlatform: 'doubao'
  },
  qwen: {
    id: 'qwen',
    label: '千问',
    defaultBaseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    baseUrlPlaceholder: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    modelPlatform: 'qwen'
  },
  kimi: {
    id: 'kimi',
    label: 'Kimi',
    defaultBaseUrl: 'https://api.moonshot.cn/v1',
    baseUrlPlaceholder: 'https://api.moonshot.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    modelPlatform: 'moonshot'
  },
  deepseek: {
    id: 'deepseek',
    label: 'DeepSeek',
    defaultBaseUrl: 'https://api.deepseek.com',
    baseUrlPlaceholder: 'https://api.deepseek.com',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    modelPlatform: 'deepseek'
  }
}

export const VIDEO_ACCOUNT_PLATFORM_METADATA: Record<
  VideoAccountPlatform,
  VideoAccountPlatformMetadata
> = {
  kling: {
    id: 'kling',
    label: 'K-Ling',
    defaultBaseUrl: 'https://app.ppapi.ai/v1',
    baseUrlPlaceholder: 'https://app.ppapi.ai/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  happyhourse: {
    id: 'happyhourse',
    label: 'Happy-Hourse',
    defaultBaseUrl: 'https://app.ppapi.ai/v1',
    baseUrlPlaceholder: 'https://app.ppapi.ai/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  seedance: {
    id: 'seedance',
    label: 'Seedance',
    defaultBaseUrl: 'https://app.ppapi.ai/v1',
    baseUrlPlaceholder: 'https://app.ppapi.ai/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  bytedance: {
    id: 'bytedance',
    label: 'ByteDance',
    defaultBaseUrl: 'https://api.modelverse.cn/v1',
    baseUrlPlaceholder: 'https://api.modelverse.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  wan3: {
    id: 'wan3',
    label: 'Wan3.0',
    defaultBaseUrl: 'https://api.modelverse.cn/v1',
    baseUrlPlaceholder: 'https://api.modelverse.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  'minimax-h3': {
    id: 'minimax-h3',
    label: 'MiniMax-H3',
    defaultBaseUrl: 'https://api.modelverse.cn/v1',
    baseUrlPlaceholder: 'https://api.modelverse.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  'pixverse-v6': {
    id: 'pixverse-v6',
    label: 'Pixverse-V6',
    defaultBaseUrl: 'https://api.modelverse.cn/v1',
    baseUrlPlaceholder: 'https://api.modelverse.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  'grok-imagine-video': {
    id: 'grok-imagine-video',
    label: 'Grok Imagine Video',
    defaultBaseUrl: 'https://api.modelverse.cn/v1',
    baseUrlPlaceholder: 'https://api.modelverse.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  kuaishou: {
    id: 'kuaishou',
    label: 'Kuaishou',
    defaultBaseUrl: 'https://api.modelverse.cn/v1',
    baseUrlPlaceholder: 'https://api.modelverse.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  }
}

export const AUDIO_ACCOUNT_PLATFORM_METADATA: Record<
  AudioAccountPlatform,
  AudioAccountPlatformMetadata
> = {
  'minimax-speech': {
    id: 'minimax-speech',
    label: 'MiniMax-Speech',
    defaultBaseUrl: 'https://api.modelverse.cn/v1',
    baseUrlPlaceholder: 'https://api.modelverse.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  'qwen-tts': {
    id: 'qwen-tts',
    label: 'Qwen TTS',
    defaultBaseUrl: 'https://api.modelverse.cn/v1',
    baseUrlPlaceholder: 'https://api.modelverse.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  }
}

export function isProviderPlatform(platform: string): platform is ProviderPlatform {
  return (PROVIDER_PLATFORMS as readonly string[]).includes(platform)
}

export function isVideoAccountPlatform(platform: string): platform is VideoAccountPlatform {
  return (VIDEO_ACCOUNT_PLATFORMS as readonly string[]).includes(platform)
}

export function isAudioAccountPlatform(platform: string): platform is AudioAccountPlatform {
  return (AUDIO_ACCOUNT_PLATFORMS as readonly string[]).includes(platform)
}

export function getPlatformMetadata(platform: string): ProviderPlatformMetadata {
  if (isProviderPlatform(platform)) {
    return PROVIDER_PLATFORM_METADATA[platform]
  }

  throw new Error(`Unknown provider platform: ${platform}`)
}

export function getVideoAccountPlatformMetadata(
  platform: VideoAccountPlatform
): VideoAccountPlatformMetadata {
  return VIDEO_ACCOUNT_PLATFORM_METADATA[platform]
}

export function getAudioAccountPlatformMetadata(
  platform: AudioAccountPlatform
): AudioAccountPlatformMetadata {
  return AUDIO_ACCOUNT_PLATFORM_METADATA[platform]
}
