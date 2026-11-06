import { describe, expect, it } from 'vitest'

import { platformBadgeClass, platformLabel } from '../platformColors'

describe('video account platform colors', () => {
  it.each([
    ['88api-video', '88API Video', 'emerald'],
    ['kling', 'K-Ling', 'sky'],
    ['happyhourse', 'Happy-Hourse', 'emerald'],
    ['seedance', 'Seedance', 'teal'],
    ['ctmo-seedance', '浙创 OPC Seedance', 'teal'],
	['xingapi-seedance', '星光 API Seedance', 'amber'],
    ['guangying', '光影 AI Seedance', 'rose'],
    ['opone', 'OPONE Seedance', 'indigo'],
    ['bytedance', 'ByteDance', 'violet'],
    ['wan3', 'Wan3.0', 'fuchsia'],
    ['minimax-h3', 'MiniMax-H3', 'pink'],
    ['pixverse-v6', 'Pixverse-V6', 'lime'],
    ['grok-imagine-video', 'Grok Imagine Video', 'cyan'],
    ['kuaishou', 'Kuaishou', 'sky']
  ])('labels and colors %s', (platform, label, color) => {
    expect(platformLabel(platform)).toBe(label)
    expect(platformBadgeClass(platform)).toContain(color)
  })
})
