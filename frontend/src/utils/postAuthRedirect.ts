/**
 * 登录 / 注册完成后的跳转目标处理。
 */

/**
 * 只允许站内相对路径（以单个 "/" 开头），拒绝 "//host"、"http://…"、"/\host" 等可能跳到站外的写法。
 * 不合法时返回 fallback。
 */
export function safeRedirectPath(raw: unknown, fallback = '/dashboard'): string {
  if (typeof raw !== 'string') return fallback
  const value = raw.trim()
  if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return fallback
  if (/[\u0000-\u001f]/.test(value)) return fallback
  return value
}

/** 登录 / 注册页互相跳转时保留 redirect 参数 */
export function redirectQuery(raw: unknown): { redirect?: string } {
  const path = safeRedirectPath(raw, '')
  return path ? { redirect: path } : {}
}
