/**
 * 折扣兑换码的换算工具。
 *
 * 折扣以「折」为单位：6 表示六折，6.5 表示六五折，10 表示不打折。
 * 客户实付 pay 元、折扣 d 折，对应积分面值 = pay / (d / 10)。
 */

const roundCents = (value: number) => Math.round(value * 100) / 100

/** 去掉多余的小数 0：6.00 -> "6"，1666.67 -> "1666.67"。 */
export const formatPlainNumber = (value: number) => String(roundCents(value))

/** 折扣以「折」为单位，合法范围 [1, 10]：低于 1 折基本是把 0.6 当成六折的误填，直接拒绝。 */
export const isValidDiscount = (discount: number) =>
  Number.isFinite(discount) && discount >= 1 && discount <= 10

/** 折扣看起来像「比例」而不是「折」（0 < d < 1，如把六折填成 0.6）。 */
export const looksLikeRatio = (discount: number) =>
  Number.isFinite(discount) && discount > 0 && discount < 1

/** 按实付金额和折扣换算积分面值（保留两位小数）；输入无效时返回 0。 */
export const creditsFromDiscount = (payAmount: number, discount: number) => {
  if (!Number.isFinite(payAmount) || payAmount <= 0 || !isValidDiscount(discount)) {
    return 0
  }
  // 先乘后除，避免 d/10 引入 0.6 这类浮点误差
  return roundCents((payAmount * 10) / discount)
}

/** 由实付金额和积分面值反推实际折扣（折，保留两位小数）；输入无效时返回 0。 */
export const effectiveDiscount = (payAmount: number, credits: number) => {
  if (!Number.isFinite(payAmount) || !Number.isFinite(credits) || payAmount <= 0 || credits <= 0) {
    return 0
  }
  return roundCents((payAmount / credits) * 10)
}

/** 生成写入兑换码备注的说明，如「张三 · 实付 1000，面值 1666.67（6折）」。 */
export const buildDiscountNote = (customerNote: string, payAmount: number, credits: number) => {
  const discount = effectiveDiscount(payAmount, credits)
  const summary = `实付 ${formatPlainNumber(payAmount)}，面值 ${formatPlainNumber(credits)}（${formatPlainNumber(discount)}折）`
  return [customerNote.trim(), summary].filter(Boolean).join(' · ')
}
