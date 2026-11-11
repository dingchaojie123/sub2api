import { describe, expect, it } from 'vitest'
import { redirectQuery, safeRedirectPath } from '../postAuthRedirect'

describe('safeRedirectPath', () => {
  it('keeps same-site absolute paths, including query strings', () => {
    expect(safeRedirectPath('/organizations?invite=abc')).toBe('/organizations?invite=abc')
  })

  it.each([
    ['protocol-relative', '//evil.com'],
    ['absolute url', 'https://evil.com'],
    ['backslash trick', '/\\evil.com'],
    ['relative', 'organizations'],
    ['control character', '/a\nb'],
    ['non-string', 42],
    ['empty', '']
  ])('falls back for %s', (_name, value) => {
    expect(safeRedirectPath(value)).toBe('/dashboard')
  })

  it('supports a custom fallback', () => {
    expect(safeRedirectPath(undefined, '')).toBe('')
  })
})

describe('redirectQuery', () => {
  it('returns the redirect when safe and nothing otherwise', () => {
    expect(redirectQuery('/organizations?invite=abc')).toEqual({ redirect: '/organizations?invite=abc' })
    expect(redirectQuery('//evil.com')).toEqual({})
    expect(redirectQuery(undefined)).toEqual({})
  })
})
