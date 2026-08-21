import { describe, expect, it } from 'vitest'
import {
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
    expect(VIDEO_ACCOUNT_PLATFORMS).toEqual(['kling', 'happyhourse', 'seedance'])

    for (const platform of VIDEO_ACCOUNT_PLATFORMS) {
      const metadata = getVideoAccountPlatformMetadata(platform)

      expect(metadata.defaultBaseUrl).toBe('https://app.ppapi.ai/v1')
      expect(metadata.authScheme).toBe('bearer')
      expect(metadata.accountType).toBe('apikey')
      expect(isVideoAccountPlatform(platform)).toBe(true)
    }
  })
})
