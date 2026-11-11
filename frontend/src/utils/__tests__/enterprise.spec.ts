import { describe, expect, it } from 'vitest'
import {
  emptyEnterpriseInput,
  enterpriseErrorMessage,
  isValidContactPhone,
  isValidCreditCode,
  normalizeEnterpriseInput,
  validateEnterpriseInput
} from '../enterprise'

const validInput = () => ({
  company_name: '示例科技有限公司',
  credit_code: '91350100M000100Y43',
  address: '上海市浦东新区世纪大道 100 号',
  contact_name: '张三',
  contact_phone: '13800138000'
})

describe('isValidCreditCode', () => {
  it('accepts well-formed codes with a correct check digit', () => {
    expect(isValidCreditCode('91350100M000100Y43')).toBe(true)
    expect(isValidCreditCode('91110000600001760P')).toBe(true)
  })

  it('normalizes case and whitespace before checking', () => {
    expect(isValidCreditCode(' 91350100m000100y43 ')).toBe(true)
  })

  it.each([
    ['empty', ''],
    ['wrong check digit', '91350100M000100Y44'],
    ['too short', '91350100M000100Y4'],
    ['too long', '91350100M000100Y433'],
    ['illegal letter I', '91350100I000100Y43'],
    ['illegal letter O', '91350100O000100Y43']
  ])('rejects %s', (_name, code) => {
    expect(isValidCreditCode(code)).toBe(false)
  })
})

describe('isValidContactPhone', () => {
  it.each(['13800138000', '138 0013 8000', '021-12345678', '02112345678', '0755-1234567-88', '12345678'])(
    'accepts %s',
    (phone) => expect(isValidContactPhone(phone)).toBe(true)
  )

  it.each(['', '12345', '23800138000', 'abcdefghijk'])('rejects %s', (phone) =>
    expect(isValidContactPhone(phone)).toBe(false)
  )
})

describe('validateEnterpriseInput', () => {
  it('returns no errors for a valid input', () => {
    expect(validateEnterpriseInput(validInput())).toEqual({})
  })

  it('reports every invalid field with its error key', () => {
    expect(validateEnterpriseInput(emptyEnterpriseInput())).toEqual({
      company_name: 'companyName',
      credit_code: 'creditCode',
      address: 'address',
      contact_name: 'contactName',
      contact_phone: 'contactPhone'
    })
  })

  it('counts characters, not UTF-16 units', () => {
    expect(validateEnterpriseInput({ ...validInput(), contact_name: '李' }).contact_name).toBe('contactName')
    expect(validateEnterpriseInput({ ...validInput(), contact_name: '李四' }).contact_name).toBeUndefined()
  })
})

describe('normalizeEnterpriseInput', () => {
  it('trims fields, upper-cases the credit code and strips phone spaces', () => {
    expect(
      normalizeEnterpriseInput({
        company_name: '  示例科技有限公司 ',
        credit_code: ' 91350100m000100y43',
        address: ' 上海市浦东新区世纪大道 100 号 ',
        contact_name: ' 张三 ',
        contact_phone: '138 0013 8000'
      })
    ).toEqual({
      company_name: '示例科技有限公司',
      credit_code: '91350100M000100Y43',
      address: '上海市浦东新区世纪大道 100 号',
      contact_name: '张三',
      contact_phone: '13800138000'
    })
  })
})

describe('enterpriseErrorMessage', () => {
  const messages: Record<string, string> = {
    'enterprise.serverErrors.ENTERPRISE_CREDIT_CODE_EXISTS': '该统一社会信用代码已被注册'
  }
  const t = (key: string) => messages[key] ?? key
  const te = (key: string) => key in messages

  it('translates known reason codes', () => {
    expect(enterpriseErrorMessage({ reason: 'ENTERPRISE_CREDIT_CODE_EXISTS', message: 'x' }, t, te, 'fb')).toBe(
      '该统一社会信用代码已被注册'
    )
  })

  it('falls back to the backend message, then to the fallback', () => {
    expect(enterpriseErrorMessage({ reason: 'UNKNOWN', message: 'backend says no' }, t, te, 'fb')).toBe('backend says no')
    expect(enterpriseErrorMessage({}, t, te, 'fb')).toBe('fb')
  })
})
