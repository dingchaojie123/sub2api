import { describe, expect, it } from 'vitest'
import {
  API_DOC_DEFAULT_SLUG,
  apiDocGroups,
  apiDocPages,
  findApiDocPage,
  resolveApiDocSlug,
} from '../apiDocsCatalog'

describe('apiDocsCatalog', () => {
  it('registers every page once, with content, and a valid default', () => {
    const slugs = apiDocPages.map((page) => page.slug)
    const paths = apiDocPages.map((page) => page.path)

    expect(new Set(slugs).size).toBe(slugs.length)
    expect(new Set(paths).size).toBe(paths.length)
    expect(findApiDocPage(API_DOC_DEFAULT_SLUG)).toBeDefined()
    apiDocPages.forEach((page) => {
      expect(page.markdown.trim().length, page.slug).toBeGreaterThan(100)
      expect(apiDocGroups.some((group) => group.id === page.group), page.slug).toBe(true)
    })
    // Seedance2.5-480全参 lives inside the Seedance (other models) page, not on a page of its own
    expect(findApiDocPage('mai-token-seedance')).toBeUndefined()
    expect(findApiDocPage('seedance-series')?.markdown).toContain('Seedance2.5-480全参')
    expect(findApiDocPage('unified')).toMatchObject({ group: 'video' })
    expect(findApiDocPage('seedance-hub')).toMatchObject({ group: 'video' })
    expect(findApiDocPage('seedance-hub')?.markdown).toContain('Seedance2.5--720p--V--按次--X')
    expect(findApiDocPage('wan')).toMatchObject({ group: 'video-v1' })
  })

  it('resolves relative links between pages', () => {
    expect(resolveApiDocSlug('PP_VIDEO_GENERATION_API.md', 'video-api/wan.md')).toBe('wan')
    expect(resolveApiDocSlug('PP_VIDEO_GENERATION_API.md', 'video-api/mai-token-seedance.md')).toBeNull()
    expect(resolveApiDocSlug('PP_VIDEO_GENERATION_API.md', 'QWEN_TTS_API.md')).toBe('qwen-tts')
    expect(resolveApiDocSlug('video-api/wan.md', '../PP_VIDEO_GENERATION_API.md')).toBe('video')
    expect(resolveApiDocSlug('video-api/wan.md', 'advanced.md')).toBe('advanced')
    expect(resolveApiDocSlug('video-api/wan.md', './advanced.md#section')).toBe('advanced')
    expect(resolveApiDocSlug('QWEN_TTS_API.md', 'PP_VIDEO_GENERATION_API.md')).toBe('video')
  })

  it('returns null for links that point nowhere or escape docs/', () => {
    expect(resolveApiDocSlug('PP_VIDEO_GENERATION_API.md', 'video-api/missing.md')).toBeNull()
    expect(resolveApiDocSlug('PP_VIDEO_GENERATION_API.md', '../README.md')).toBeNull()
    expect(resolveApiDocSlug('video-api/wan.md', '../../outside.md')).toBeNull()
    expect(resolveApiDocSlug('PP_VIDEO_GENERATION_API.md', '#only-an-anchor')).toBeNull()
  })

  it('has no dead links: every relative .md link in every page resolves to a registered page', () => {
    apiDocPages.forEach((page) => {
      const links = [...page.markdown.matchAll(/\]\(([^)\s]+\.md(?:#[^)]*)?)\)/g)].map((m) => m[1])
      links
        .filter((href) => !/^https?:\/\//i.test(href))
        .forEach((href) => {
          expect(resolveApiDocSlug(page.path, href), `${page.slug} -> ${href}`).not.toBeNull()
        })
    })
  })

  it('exposes no upstream or internal wording in any customer page', () => {
    const forbidden = [
      '浙创', 'OPC', '光影', 'OPONE', '88API', '88api', '优云智算', '火山', '方舟', 'ByteDance', 'bytedance',
      '星光', 'CTMO', 'ctmo', 'xingapi', 'guangying', 'opone', 'compshare', 'ModelVerse', 'gy-model', 'sdtj', '极九', 'jijiu',
      '管理员', '后台', '渠道计费', '分组倍率', 'docs/internal', 'PLATFORM_ADMIN',
    ]
    apiDocPages.forEach((page) => {
      forbidden.forEach((word) => {
        expect(page.markdown.includes(word), `${page.slug} mentions "${word}"`).toBe(false)
      })
      expect(page.markdown).not.toContain('api.mai-token.com')
    })
  })

  it('does not list models that are no longer sold', () => {
    const retired = [
      'seedance2.0-933-Q', 'kling-3.0-turbo', 'SD2.0', 'SD2.5', 'seedance-2.5-b1', 'seedance-2.5-SS', 'seedance2.5-one-720p',
      'seedance-2.0-max', 'seedance-2.5-720-max',
    ]
    const all = apiDocPages.map((page) => page.markdown).join('\n')
    retired.forEach((id) => expect(all.includes(id), id).toBe(false))
  })
})
