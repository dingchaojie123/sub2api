import { describe, expect, it } from 'vitest'

import enAdminResources from '../locales/en/admin/resources'
import zhCommon from '../locales/zh/common'
import zhDashboard from '../locales/zh/dashboard'
import zhAdminResources from '../locales/zh/admin/resources'

describe('lottery Chinese locale naming', () => {
  it('uses Lucky Blind Box wording for the user-facing lottery entry', () => {
    expect(zhCommon.nav.lottery).toBe('幸运盲盒')
    expect(zhDashboard.lottery.title).toBe('幸运盲盒')
  })

  it('uses the updated prize amounts in admin lottery eligibility hints', () => {
    expect(zhAdminResources.redeem.lottery.ineligibleSelectedHint).toContain('$30/$10/$5/$2')
    expect(enAdminResources.redeem.lottery.ineligibleSelectedHint).toContain('$30/$10/$5/$2')
  })
})
