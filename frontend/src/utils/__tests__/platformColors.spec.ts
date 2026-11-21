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
	['jijiu-seedance', '极九 API Seedance', 'orange'],
	['miaowu-seedance', '喵呜 API Seedance', 'lime'],
	['paisio-seedance', 'Paisio Seedance', 'blue'],
	['apophis-seedance', 'Apophis Seedance', 'yellow'],
	['xsn-seedance', 'XSN Seedance', 'red'],
	['fengyue-seedance', '风月 Seedance', 'pink'],
	['hongpei-seedance', '红佩 Seedance', 'violet'],
	['vshi-seedance', 'VSHI Seedance', 'teal'],
	['huahuo-seedance', 'HUAHUO Seedance', 'fuchsia'],
	['zexi-seedance', '泽西 Seedance', 'stone'],
	['xm-seedance', 'XM Seedance', 'sky'],
	['naiwa-seedance', '奶蛙 Seedance', 'emerald'],
	['v6k-seedance', 'V6K Seedance', 'indigo'],
	['kyy-wan', '客易云 Wan3.0', 'indigo'],
    ['guangying', '光影 AI Seedance', 'rose'],
    ['opone', 'OPONE Seedance', 'indigo'],
    ['mai-token-seedance', 'Mai Token Seedance', 'cyan'],
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
