import { describe, expect, it } from 'vitest'

import { platformBadgeClass, platformLabel } from '../platformColors'

describe('video account platform colors', () => {
  it.each([
    ['kling', 'K-Ling', 'sky'],
    ['happyhourse', 'Happy-Hourse', 'emerald'],
    ['seedance', 'Seedance', 'teal'],
    ['bytedance', 'ByteDance', 'violet'],
    ['wan3', 'Wan3.0', 'fuchsia'],
    ['minimax-h3', 'MiniMax-H3', 'pink'],
    ['pixverse-v6', 'Pixverse-V6', 'lime']
  ])('labels and colors %s', (platform, label, color) => {
    expect(platformLabel(platform)).toBe(label)
    expect(platformBadgeClass(platform)).toContain(color)
  })
})
