export const PROVIDER_PLATFORMS = ['doubao', 'qwen', 'kimi', 'deepseek', 'midjourney'] as const

export type ProviderPlatform = (typeof PROVIDER_PLATFORMS)[number]

export const VIDEO_ACCOUNT_PLATFORMS = [
  '88api-video',
  'kling',
  'happyhourse',
  'seedance',
  'ctmo-seedance',
	'xingapi-seedance',
	'jijiu-seedance',
	'miaowu-seedance',
	'paisio-seedance',
	'apophis-seedance',
	'xsn-seedance',
	'fengyue-seedance',
	'hongpei-seedance',
	'vshi-seedance',
	'huahuo-seedance',
	'zexi-seedance',
	'xm-seedance',
	'naiwa-seedance',
	'v6k-seedance',
  'mai-token-seedance',
  'guangying',
  'opone',
  'bytedance',
  'wan3',
  'kyy-wan',
  'minimax-h3',
  'minimax-h3-compshare',
  'pixverse-v6',
  'grok-imagine-video',
  'kuaishou'
] as const

export type VideoAccountPlatform = (typeof VIDEO_ACCOUNT_PLATFORMS)[number]

export const AUDIO_ACCOUNT_PLATFORMS = ['minimax-speech', 'qwen-tts'] as const

export type AudioAccountPlatform = (typeof AUDIO_ACCOUNT_PLATFORMS)[number]

export const CHANNEL_PRICING_PLATFORMS = [
  '88api-video',
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
  'midjourney',
  'kling',
  'happyhourse',
  'seedance',
  'ctmo-seedance',
	'xingapi-seedance',
	'jijiu-seedance',
	'miaowu-seedance',
	'paisio-seedance',
	'apophis-seedance',
	'xsn-seedance',
	'fengyue-seedance',
	'hongpei-seedance',
	'vshi-seedance',
	'huahuo-seedance',
	'zexi-seedance',
	'xm-seedance',
	'naiwa-seedance',
	'v6k-seedance',
  'mai-token-seedance',
  'seedance-hub',
  'guangying',
  'opone',
  'bytedance',
  'wan3',
  'kyy-wan',
  'minimax-h3',
  'minimax-h3-compshare',
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
  },
  midjourney: {
    id: 'midjourney',
    label: 'Midjourney',
    defaultBaseUrl: 'https://api.modelverse.cn/v1',
    baseUrlPlaceholder: 'https://api.modelverse.cn/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    modelPlatform: 'midjourney'
  }
}

export const VIDEO_ACCOUNT_PLATFORM_METADATA: Record<
  VideoAccountPlatform,
  VideoAccountPlatformMetadata
> = {
  '88api-video': {
    id: '88api-video',
    label: '88API Video',
    defaultBaseUrl: 'https://88api.ai/v1',
    baseUrlPlaceholder: 'https://88api.ai/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
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
  'ctmo-seedance': {
    id: 'ctmo-seedance',
    label: '浙创 OPC Seedance',
    defaultBaseUrl: 'https://video.ctmoai.com',
    baseUrlPlaceholder: 'https://video.ctmoai.com',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
	'xingapi-seedance': {
		id: 'xingapi-seedance',
		label: '星光 API Seedance',
		defaultBaseUrl: 'https://xingapi.top/v1',
		baseUrlPlaceholder: 'https://xingapi.top/v1',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'jijiu-seedance': {
		id: 'jijiu-seedance',
		label: '极九 API Seedance',
		defaultBaseUrl: 'https://newapi.jijiucanvas.com',
		baseUrlPlaceholder: 'https://newapi.jijiucanvas.com',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'miaowu-seedance': {
		id: 'miaowu-seedance',
		label: '喵呜 API Seedance',
		defaultBaseUrl: 'https://api.miaowuai.store/v1',
		baseUrlPlaceholder: 'https://api.miaowuai.store/v1',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'paisio-seedance': {
		id: 'paisio-seedance',
		label: 'Paisio Seedance',
		defaultBaseUrl: 'https://api.paisio.online',
		baseUrlPlaceholder: 'https://api.paisio.online',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'apophis-seedance': {
		id: 'apophis-seedance',
		label: 'Apophis Seedance',
		defaultBaseUrl: 'https://www.apophiscode.com',
		baseUrlPlaceholder: 'https://www.apophiscode.com',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'xsn-seedance': {
		id: 'xsn-seedance',
		label: 'XSN Seedance',
		defaultBaseUrl: 'https://xsn686.cn',
		baseUrlPlaceholder: 'https://xsn686.cn',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'fengyue-seedance': {
		id: 'fengyue-seedance',
		label: '风月 Seedance',
		defaultBaseUrl: 'https://api.nldw.online',
		baseUrlPlaceholder: 'https://api.nldw.online',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'hongpei-seedance': {
		id: 'hongpei-seedance',
		label: '红佩 Seedance',
		defaultBaseUrl: 'https://api.hongpei.click',
		baseUrlPlaceholder: 'https://api.hongpei.click',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'vshi-seedance': {
		id: 'vshi-seedance',
		label: 'VSHI Seedance',
		defaultBaseUrl: 'https://api.vshiai.com',
		baseUrlPlaceholder: 'https://api.vshiai.com',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'huahuo-seedance': {
		id: 'huahuo-seedance',
		label: 'HUAHUO Seedance',
		defaultBaseUrl: 'https://huahuo.ccwu.cc',
		baseUrlPlaceholder: 'https://huahuo.ccwu.cc',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'zexi-seedance': {
		id: 'zexi-seedance',
		label: '泽西 Seedance',
		defaultBaseUrl: 'https://zexitongxue.com',
		baseUrlPlaceholder: 'https://zexitongxue.com',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'xm-seedance': {
		id: 'xm-seedance',
		label: 'XM Seedance',
		defaultBaseUrl: 'https://ai.987xm.com',
		baseUrlPlaceholder: 'https://ai.987xm.com',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'naiwa-seedance': {
		id: 'naiwa-seedance',
		label: '奶蛙 Seedance',
		defaultBaseUrl: 'https://api.nailongapi.cc',
		baseUrlPlaceholder: 'https://api.nailongapi.cc',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
	'v6k-seedance': {
		id: 'v6k-seedance',
		label: 'V6K Seedance',
		defaultBaseUrl: 'https://www.v6k.top',
		baseUrlPlaceholder: 'https://www.v6k.top',
		apiKeyPlaceholder: 'sk-...',
		accountType: 'apikey',
		authScheme: 'bearer'
	},
  'mai-token-seedance': {
    id: 'mai-token-seedance',
    label: 'Mai Token Seedance',
    defaultBaseUrl: 'https://api.mai-token.com',
    baseUrlPlaceholder: 'https://api.mai-token.com',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  guangying: {
    id: 'guangying',
    label: '光影 AI Seedance',
    defaultBaseUrl: 'https://api.guangying.store',
    baseUrlPlaceholder: 'https://api.guangying.store',
    apiKeyPlaceholder: 'sk-live-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  opone: {
    id: 'opone',
    label: 'OPONE Seedance',
    defaultBaseUrl: 'https://api.opone.cc',
    baseUrlPlaceholder: 'https://api.opone.cc',
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
  'kyy-wan': {
    id: 'kyy-wan',
    label: '客易云 Wan3.0',
    defaultBaseUrl: 'https://zcbservice.aizfw.cn/kyyReactApiServer',
    baseUrlPlaceholder: 'https://zcbservice.aizfw.cn/kyyReactApiServer',
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
  'minimax-h3-compshare': {
    id: 'minimax-h3-compshare',
    label: 'MiniMax-H3（优云智算）',
    defaultBaseUrl: 'https://cp.compshare.cn',
    baseUrlPlaceholder: 'https://cp.compshare.cn',
    apiKeyPlaceholder: 'sk-ml-...',
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

/**
 * Seedance upstream platforms whose lines a Seedance aggregate group serves (and that can have a
 * line route, see GroupLineRoutesModal). The accounts of these platforms can be bound to an
 * aggregate group, and, as backup accounts for a line route, to the ordinary group of another of
 * these platforms. Keep in sync with seedanceHubMemberPlatforms in the backend
 * (service/pp_video_seedance_hub.go).
 */
export const SEEDANCE_HUB_MEMBER_PLATFORMS: readonly string[] = [
  'ctmo-seedance',
  'guangying',
  'xingapi-seedance',
  'mai-token-seedance',
  'jijiu-seedance',
  'miaowu-seedance',
  'paisio-seedance',
  'apophis-seedance',
  'xsn-seedance',
  'fengyue-seedance',
  'hongpei-seedance',
  'vshi-seedance',
  'huahuo-seedance',
  'zexi-seedance',
  'xm-seedance',
  'naiwa-seedance',
  'v6k-seedance'
]

/** Whether a group of this platform can carry line routes: an aggregate group, or the ordinary group of a Seedance platform. */
export const supportsLineRoutes = (groupPlatform: string): boolean =>
  groupPlatform === 'seedance-hub' || SEEDANCE_HUB_MEMBER_PLATFORMS.includes(groupPlatform)
