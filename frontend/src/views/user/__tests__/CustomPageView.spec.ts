import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../CustomPageView.vue')
const viewSource = readFileSync(viewPath, 'utf8')

describe('CustomPageView external menu mode', () => {
  it('renders external custom menu items without iframe embedding', () => {
    expect(viewSource).toContain("menuItem.value?.open_mode === 'external'")
    expect(viewSource).toContain('sanitizeUrl(menuItem.value.url)')
    expect(viewSource).toContain('v-else-if="isExternalMode"')
    expect(viewSource).toContain('isExternalMode.value) return')
  })
})

describe('CustomPageView iframe fallback', () => {
  it('shows an actionable fallback when iframe content does not load', () => {
    expect(viewSource).toContain('showEmbedFallback')
    expect(viewSource).toContain('customPage.embedBlockedDesc')
    expect(viewSource).toContain('@load="handleIframeLoad"')
    expect(viewSource).toContain('resetIframeFallback')
  })
})
