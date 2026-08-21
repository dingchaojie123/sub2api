import { describe, expect, it } from 'vitest'

import { platformBadgeClass, platformLabel } from '../platformColors'

describe('video account platform colors', () => {
  it.each([
    ['kling', 'K-Ling', 'sky'],
    ['happyhourse', 'Happy-Hourse', 'emerald'],
    ['seedance', 'Seedance', 'teal']
  ])('labels and colors %s', (platform, label, color) => {
    expect(platformLabel(platform)).toBe(label)
    expect(platformBadgeClass(platform)).toContain(color)
  })
})
