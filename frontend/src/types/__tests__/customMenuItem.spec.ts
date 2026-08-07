import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const typesSource = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts'), 'utf8')

describe('CustomMenuItem type', () => {
  it('supports configurable opening modes for iframe-blocking external pages', () => {
    expect(typesSource).toContain("open_mode?: 'embed' | 'external'")
  })
})
