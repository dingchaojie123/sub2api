import { describe, expect, it } from 'vitest'
import { BILLING_MODE_VIDEO } from '../channel'
import { CHANNEL_PRICING_PLATFORMS } from '../platforms'

describe('channel video billing configuration', () => {
  it('exposes video billing mode and all PP video platforms', () => {
    expect(BILLING_MODE_VIDEO).toBe('video')
    expect(CHANNEL_PRICING_PLATFORMS).toEqual(
      expect.arrayContaining(['kling', 'happyhourse', 'seedance', 'ctmo-seedance', 'xingapi-seedance', 'jijiu-seedance', 'miaowu-seedance', 'paisio-seedance', 'apophis-seedance', 'xsn-seedance', 'fengyue-seedance', 'hongpei-seedance', 'vshi-seedance', 'huahuo-seedance', 'zexi-seedance', 'xm-seedance', 'naiwa-seedance', 'v6k-seedance', 'mai-token-seedance', 'seedance-hub', 'guangying', 'opone', 'bytedance', 'wan3', 'minimax-h3', 'pixverse-v6', 'grok-imagine-video', 'kuaishou'])
    )
  })
})
