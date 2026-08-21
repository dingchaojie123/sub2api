import { describe, expect, it } from 'vitest'
import { BILLING_MODE_VIDEO } from '../channel'
import { CHANNEL_PRICING_PLATFORMS } from '../platforms'

describe('channel video billing configuration', () => {
  it('exposes video billing mode and all PP video platforms', () => {
    expect(BILLING_MODE_VIDEO).toBe('video')
    expect(CHANNEL_PRICING_PLATFORMS).toEqual(
      expect.arrayContaining(['kling', 'happyhourse', 'seedance'])
    )
  })
})
