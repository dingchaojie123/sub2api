/**
 * 企业信息校验（与后端 service/enterprise.go 规则保持一致）。
 * 返回错误对应的 i18n key（相对 `enterprise.errors.`），无错误时返回空字符串。
 */
import type { EnterpriseInput } from '@/api/enterprise'

const CREDIT_CODE_CHARSET = '0123456789ABCDEFGHJKLMNPQRTUWXY'
const CREDIT_CODE_WEIGHTS = [1, 3, 9, 27, 19, 26, 16, 17, 20, 29, 25, 13, 8, 24, 10, 30, 28]
const CREDIT_CODE_PATTERN = /^[0-9A-HJ-NP-RT-UW-Y]{18}$/
const MOBILE_PATTERN = /^1[3-9][0-9]{9}$/
const LANDLINE_PATTERN = /^(0[0-9]{2,3}-?)?[0-9]{7,8}(-[0-9]{1,6})?$/

export function normalizeCreditCode(code: string): string {
  return code.replace(/\s+/g, '').toUpperCase()
}

export function normalizePhone(phone: string): string {
  return phone.replace(/\s+/g, '')
}

/** 统一社会信用代码（GB 32100-2015）：18 位、字符集、校验位 */
export function isValidCreditCode(raw: string): boolean {
  const code = normalizeCreditCode(raw)
  if (!CREDIT_CODE_PATTERN.test(code)) return false
  let sum = 0
  for (let i = 0; i < 17; i++) {
    sum += CREDIT_CODE_CHARSET.indexOf(code[i]) * CREDIT_CODE_WEIGHTS[i]
  }
  return code[17] === CREDIT_CODE_CHARSET[(31 - (sum % 31)) % 31]
}

export function isValidContactPhone(raw: string): boolean {
  const phone = normalizePhone(raw)
  return MOBILE_PATTERN.test(phone) || LANDLINE_PATTERN.test(phone)
}

function lengthBetween(value: string, min: number, max: number): boolean {
  const n = Array.from(value.trim()).length
  return n >= min && n <= max
}

export type EnterpriseFieldErrors = Partial<Record<keyof EnterpriseInput, string>>

/** 校验全部字段，返回 {字段: 错误 key}；全部通过时为空对象 */
export function validateEnterpriseInput(input: EnterpriseInput): EnterpriseFieldErrors {
  const errors: EnterpriseFieldErrors = {}
  if (!lengthBetween(input.company_name, 2, 100)) errors.company_name = 'companyName'
  if (!isValidCreditCode(input.credit_code)) errors.credit_code = 'creditCode'
  if (!lengthBetween(input.address, 5, 200)) errors.address = 'address'
  if (!lengthBetween(input.contact_name, 2, 50)) errors.contact_name = 'contactName'
  if (!isValidContactPhone(input.contact_phone)) errors.contact_phone = 'contactPhone'
  return errors
}

export function emptyEnterpriseInput(): EnterpriseInput {
  return { company_name: '', credit_code: '', address: '', contact_name: '', contact_phone: '' }
}

/** 提交时统一规范化（去空白、信用代码转大写） */
export function normalizeEnterpriseInput(input: EnterpriseInput): EnterpriseInput {
  return {
    company_name: input.company_name.trim(),
    credit_code: normalizeCreditCode(input.credit_code),
    address: input.address.trim(),
    contact_name: input.contact_name.trim(),
    contact_phone: normalizePhone(input.contact_phone)
  }
}

/**
 * 把后端错误（按 reason 码）翻译为当前语言的提示；没有对应翻译时回退到后端 message 或 fallback。
 */
export function enterpriseErrorMessage(
  error: unknown,
  t: (key: string) => string,
  te: (key: string) => boolean,
  fallback: string
): string {
  const err = (error || {}) as { reason?: string; message?: string }
  if (err.reason) {
    const key = `enterprise.serverErrors.${err.reason}`
    if (te(key)) return t(key)
  }
  return err.message || fallback
}
