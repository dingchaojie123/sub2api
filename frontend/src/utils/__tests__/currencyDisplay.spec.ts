import { describe, expect, it, vi } from 'vitest'
import { formatCurrency } from '../format'
import { formatScaled } from '../pricing'
import { formatTokenPricePerMillion } from '../usagePricing'

vi.mock('@/i18n', () => ({ getLocale: () => 'en-US', i18n: { global: { t: (key: string) => key } } }))

describe('currency display', () => {
  it('keeps amount precision and signs without dollar symbols', () => {
    expect(formatCurrency(null)).toBe('0.00')
    expect(formatCurrency(undefined)).toBe('0.00')
    expect(formatCurrency(0)).toBe('0.00')
    expect(formatCurrency(-1234.5)).toBe('-1,234.50')
    expect(formatCurrency(0.000123)).toBe('0.000123')
    expect(formatCurrency(12.5, 'CNY')).toBe('CN¥12.50')
  })

  it('keeps scaled prices and token unit prices numeric', () => {
    expect(formatScaled(0.000003, 1_000_000)).toBe('3')
    expect(formatScaled(0.5, 1)).toBe('0.5')
    expect(formatTokenPricePerMillion(3, 1_000_000)).toBe('3.0000')
    expect(formatTokenPricePerMillion(0, 1_000_000)).toBe('0.0000')
    expect(formatTokenPricePerMillion(3, 0)).toBe('-')
  })
})
