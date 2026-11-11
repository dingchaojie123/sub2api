/**
 * 团队邀请令牌解析：兼容粘贴完整邀请链接（…/organizations?invite=令牌）或直接粘贴令牌。
 */
export function extractInviteToken(raw: string): string {
  const value = raw.trim()
  const match = value.match(/[?&]invite=([^&#\s]+)/)
  if (!match) return value
  try {
    return decodeURIComponent(match[1])
  } catch {
    return match[1]
  }
}
