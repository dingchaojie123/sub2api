import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { put } = vi.hoisted(() => ({
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { put },
}))

import { setBillingSource, setTeamShare } from '@/api/keys'

describe('API key billing source API', () => {
  beforeEach(() => {
    put.mockReset()
    put.mockResolvedValue({ data: { id: 205 } })
    vi.spyOn(globalThis.crypto, 'randomUUID').mockReturnValue(
      '11111111-1111-4111-8111-111111111111'
    )
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sends organization billing parameters in the JSON body', async () => {
    await setBillingSource(205, 'organization', 12)

    expect(put).toHaveBeenCalledWith(
      '/keys/billing-source',
      { api_key_id: 205, type: 'organization', organization_id: 12 },
      {
        headers: {
          'Idempotency-Key':
            'key-billing-source-205-11111111-1111-4111-8111-111111111111',
        },
      }
    )
  })

  it('omits organization_id when switching to personal billing', async () => {
    await setBillingSource(205, 'personal')

    expect(put.mock.calls[0][1]).toEqual({ api_key_id: 205, type: 'personal' })
  })

  it('sends the team share toggle with an idempotency key', async () => {
    put.mockResolvedValue({ data: { key: { id: 205 }, issued: 2, skipped: 0 } })

    const result = await setTeamShare(205, true)

    expect(put).toHaveBeenCalledWith(
      '/keys/team-share',
      { api_key_id: 205, shared: true },
      {
        headers: {
          'Idempotency-Key': 'key-team-share-205-11111111-1111-4111-8111-111111111111',
        },
      }
    )
    expect(result.issued).toBe(2)
  })
})
