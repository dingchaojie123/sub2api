import { describe, expect, it } from 'vitest'
import {
  buildDiscountNote,
  creditsFromDiscount,
  effectiveDiscount,
  formatPlainNumber,
  isValidDiscount,
  looksLikeRatio
} from '../redeemDiscount'

describe('creditsFromDiscount', () => {
  it('converts a 6-discount recharge of 1000 to 1666.67 credits', () => {
    expect(creditsFromDiscount(1000, 6)).toBe(1666.67)
  })

  it('is exact when the division is clean', () => {
    expect(creditsFromDiscount(258, 6)).toBe(430)
    expect(creditsFromDiscount(500, 5)).toBe(1000)
    expect(creditsFromDiscount(100, 10)).toBe(100)
  })

  it('supports fractional discounts', () => {
    expect(creditsFromDiscount(650, 6.5)).toBe(1000)
  })

  it('returns 0 for invalid input', () => {
    expect(creditsFromDiscount(0, 6)).toBe(0)
    expect(creditsFromDiscount(-100, 6)).toBe(0)
    expect(creditsFromDiscount(1000, 0)).toBe(0)
    expect(creditsFromDiscount(1000, 11)).toBe(0)
    expect(creditsFromDiscount(Number.NaN, 6)).toBe(0)
    expect(creditsFromDiscount(1000, Number.NaN)).toBe(0)
  })
})

describe('isValidDiscount', () => {
  it('accepts [1, 10] only', () => {
    expect(isValidDiscount(6)).toBe(true)
    expect(isValidDiscount(10)).toBe(true)
    expect(isValidDiscount(1)).toBe(true)
    expect(isValidDiscount(0.99)).toBe(false)
    expect(isValidDiscount(0)).toBe(false)
    expect(isValidDiscount(10.01)).toBe(false)
    expect(isValidDiscount(Number.POSITIVE_INFINITY)).toBe(false)
  })

  it('rejects 0.6, the common mistake for a 60%-of-price discount', () => {
    expect(isValidDiscount(0.6)).toBe(false)
    expect(creditsFromDiscount(1000, 0.6)).toBe(0)
  })
})

describe('looksLikeRatio', () => {
  it('flags values strictly between 0 and 1', () => {
    expect(looksLikeRatio(0.6)).toBe(true)
    expect(looksLikeRatio(0.99)).toBe(true)
    expect(looksLikeRatio(1)).toBe(false)
    expect(looksLikeRatio(6)).toBe(false)
    expect(looksLikeRatio(0)).toBe(false)
    expect(looksLikeRatio(Number.NaN)).toBe(false)
  })
})

describe('effectiveDiscount', () => {
  it('derives the discount from pay amount and credits', () => {
    expect(effectiveDiscount(1000, 1666.67)).toBe(6)
    expect(effectiveDiscount(1000, 1667)).toBe(6)
    expect(effectiveDiscount(1000, 2000)).toBe(5)
  })

  it('returns 0 for invalid input', () => {
    expect(effectiveDiscount(0, 1000)).toBe(0)
    expect(effectiveDiscount(1000, 0)).toBe(0)
  })
})

describe('buildDiscountNote', () => {
  it('prefixes the customer note and trims it', () => {
    expect(buildDiscountNote('  张三  ', 1000, 1666.67)).toBe('张三 · 实付 1000，面值 1666.67（6折）')
  })

  it('omits the customer part when empty', () => {
    expect(buildDiscountNote('', 1000, 1667)).toBe('实付 1000，面值 1667（6折）')
  })
})

describe('formatPlainNumber', () => {
  it('drops trailing zeros', () => {
    expect(formatPlainNumber(6)).toBe('6')
    expect(formatPlainNumber(6.5)).toBe('6.5')
    expect(formatPlainNumber(1666.666)).toBe('1666.67')
  })
})
