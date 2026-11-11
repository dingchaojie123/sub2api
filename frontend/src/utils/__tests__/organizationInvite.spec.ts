import { describe, expect, it } from 'vitest'
import { extractInviteToken } from '../organizationInvite'

describe('extractInviteToken', () => {
  it('returns a bare token unchanged (trimmed)', () => {
    expect(extractInviteToken('  abc123  ')).toBe('abc123')
  })

  it('extracts the token from a full invitation link', () => {
    expect(extractInviteToken('https://example.com/organizations?invite=abc123')).toBe('abc123')
    expect(extractInviteToken('https://example.com/organizations?x=1&invite=abc123&y=2#top')).toBe('abc123')
  })

  it('decodes percent-encoded tokens and tolerates malformed encoding', () => {
    expect(extractInviteToken('/organizations?invite=a%2Bb')).toBe('a+b')
    expect(extractInviteToken('/organizations?invite=100%')).toBe('100%')
  })
})
