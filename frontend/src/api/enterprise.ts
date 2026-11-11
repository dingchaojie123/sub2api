/**
 * 企业账号 API：用户侧（团队空间访问权限、升级、修改企业信息）、注册预检、后台审核。
 */
import { apiClient } from './client'

export type EnterpriseStatus = 'pending' | 'approved' | 'rejected' | 'incomplete'
export type AccountType = 'personal' | 'enterprise'

export interface EnterpriseInput {
  company_name: string
  credit_code: string
  address: string
  contact_name: string
  contact_phone: string
}

export interface EnterpriseProfile extends EnterpriseInput {
  user_id: number
  verify_status: EnterpriseStatus
  reject_reason: string
  reviewed_by?: number
  reviewed_at?: string
  created_at: string
  updated_at: string
}

export interface WorkspaceAccess {
  account_type: AccountType
  enterprise: EnterpriseProfile | null
  can_create_team: boolean
  is_team_member: boolean
  has_team_access: boolean
}

export interface EnterpriseAdminItem extends EnterpriseProfile {
  email: string
  username: string
}

export interface EnterpriseAdminList {
  items: EnterpriseAdminItem[]
  total: number
  page: number
  page_size: number
  pages: number
}

export const enterpriseAPI = {
  /** 当前账号类型、企业信息、团队空间可见性 */
  async workspaceAccess(): Promise<WorkspaceAccess> {
    return (await apiClient.get('/user/workspace-access')).data
  },
  /** 个人账号升级为企业账号 */
  async upgrade(input: EnterpriseInput): Promise<EnterpriseProfile> {
    return (await apiClient.post('/user/enterprise', input)).data
  },
  /** 修改企业信息（已认证后公司名称、信用代码不可改） */
  async update(input: EnterpriseInput): Promise<EnterpriseProfile> {
    return (await apiClient.put('/user/enterprise', input)).data
  },
  /** 注册前预检：格式 + 信用代码是否已被注册（公开接口） */
  async validateForRegistration(input: EnterpriseInput): Promise<{ valid: boolean }> {
    return (await apiClient.post('/auth/validate-enterprise', input)).data
  }
}

export const adminEnterpriseAPI = {
  async list(params: { page?: number; page_size?: number; status?: string; search?: string }): Promise<EnterpriseAdminList> {
    return (await apiClient.get('/admin/enterprises', { params })).data
  },
  async review(userId: number, action: 'approve' | 'reject', reason?: string): Promise<EnterpriseProfile> {
    return (await apiClient.post(`/admin/enterprises/${userId}/review`, { action, reason })).data
  }
}
