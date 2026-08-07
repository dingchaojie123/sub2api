import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDir = dirname(fileURLToPath(import.meta.url))
const routerSource = readFileSync(resolve(testDir, '../index.ts'), 'utf8')
const sidebarSource = readFileSync(
  resolve(testDir, '../../components/layout/AppSidebar.vue'),
  'utf8',
)

describe('model pricing frontend route', () => {
  it('does not use /models, which is reserved for the OpenAI-compatible API', () => {
    expect(routerSource).not.toContain("path: '/models'")
    expect(sidebarSource).not.toContain("path: '/models'")
  })

  it('exposes the model pricing page on a non-API path', () => {
    expect(routerSource).toContain("path: '/model-pricing'")
    expect(sidebarSource).toContain("path: '/model-pricing'")
  })
})
