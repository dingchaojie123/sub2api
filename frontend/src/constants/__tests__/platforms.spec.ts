import { describe, expect, it } from 'vitest'
import {
  AUDIO_ACCOUNT_PLATFORMS,
  PROVIDER_PLATFORMS,
  VIDEO_ACCOUNT_PLATFORMS,
  getPlatformMetadata,
  getVideoAccountPlatformMetadata,
  isProviderPlatform,
  isVideoAccountPlatform
} from '../platforms'

describe('provider platform metadata', () => {
  it('defines the four independent OpenAI-compatible provider platforms', () => {
    expect(PROVIDER_PLATFORMS).toEqual(['doubao', 'qwen', 'kimi', 'deepseek'])

    for (const platform of PROVIDER_PLATFORMS) {
      const metadata = getPlatformMetadata(platform)

      expect(metadata.label).toBeTruthy()
      expect(metadata.defaultBaseUrl).toMatch(/^https:\/\//)
      expect(metadata.accountType).toBe('apikey')
      expect(metadata.modelPlatform).toBeTruthy()
      expect(isProviderPlatform(platform)).toBe(true)
    }
  })
})

describe('video account platform metadata', () => {
  it('defines video platforms with PP API defaults and Bearer auth', () => {
    expect(VIDEO_ACCOUNT_PLATFORMS).toEqual([
      'kling',
      'happyhourse',
      'seedance',
      'bytedance',
      'wan3',
      'minimax-h3',
      'pixverse-v6',
      'grok-imagine-video',
      'kuaishou'
    ])

    for (const platform of VIDEO_ACCOUNT_PLATFORMS) {
      const metadata = getVideoAccountPlatformMetadata(platform)

      const expectedBaseUrl =
        platform === 'bytedance' ||
        platform === 'wan3' ||
        platform === 'minimax-h3' ||
        platform === 'pixverse-v6' ||
        platform === 'grok-imagine-video' ||
        platform === 'kuaishou'
          ? 'https://api.modelverse.cn/v1'
          : 'https://app.ppapi.ai/v1'

      expect(metadata.defaultBaseUrl).toBe(expectedBaseUrl)
      expect(metadata.baseUrlPlaceholder).toBe(expectedBaseUrl)
      expect(metadata.authScheme).toBe('bearer')
      expect(metadata.accountType).toBe('apikey')
      expect(isVideoAccountPlatform(platform)).toBe(true)
    }
  })
})

describe('audio account platform metadata', () => {
  it('defines audio platforms with ModelVerse defaults and Bearer auth', () => {
    expect(AUDIO_ACCOUNT_PLATFORMS).toEqual(['minimax-speech', 'qwen-tts'])
  })
})
