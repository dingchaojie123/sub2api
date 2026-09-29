import { describe, expect, it } from 'vitest'
import { currencySymbol, formatPaymentAmount } from '../currency'

describe('formatPaymentAmount', () => {
  it.each(['en-US', 'zh-CN', 'de-DE'])('omits dollar symbols in %s', (locale) => {
    for (const currency of ['USD', 'HKD', 'TWD', 'AUD', 'CAD', 'SGD', 'NZD', 'MOP']) {
      expect(formatPaymentAmount(12.5, currency, locale)).not.toContain('$')
      expect(currencySymbol(currency)).not.toContain('$')
    }
    expect(formatPaymentAmount(12.5, 'USD', 'en-US')).toBe('12.50')
    expect(formatPaymentAmount(12.5, 'CNY', 'en-US')).toBe('¥12.50')
    expect(formatPaymentAmount(Number.NaN, 'USD', locale)).not.toContain('NaN')
  })

  it('uses the currency default fraction digits', () => {
    expect(formatPaymentAmount(100, 'JPY', 'en-US')).not.toContain('.00')
    expect(formatPaymentAmount(100, 'KRW', 'en-US')).not.toContain('.00')
    expect(formatPaymentAmount(100, 'HKD', 'en-US')).toContain('.00')
  })
})

describe('currencySymbol', () => {
  it('maps common payment currencies and falls back safely', () => {
    expect(currencySymbol('USD')).toBe('')
    expect(currencySymbol('cny')).toBe('¥')
    expect(currencySymbol('EUR')).toBe('€')
    expect(currencySymbol('')).toBe('¥')
    expect(currencySymbol('XYZ')).toBe('XYZ')
  })
})
