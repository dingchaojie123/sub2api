import { apiClient } from './client'

export type OrganizationRole = 'owner' | 'admin' | 'member'

export interface Organization {
  id: number
  name: string
  owner_user_id: number
  status: string
  seat_limit: number
  seat_used: number
  balance: number
  display_balance: number
  frozen_balance: number
  frozen_display_balance: number
  role: OrganizationRole
  created_at: string
}

export interface OrganizationMember {
  id: number
  organization_id: number
  user_id: number
  email: string
  username: string
  role: OrganizationRole
  status: 'active' | 'disabled'
  monthly_limit: number
  monthly_used: number
  usage_period_start: string
  joined_at: string
}

export interface OrganizationInvitation {
  id: number
  email?: string
  role: 'admin' | 'member'
  max_uses: number
  used_count: number
  expires_at: string
  created_at: string
  token?: string
}

export interface OrganizationUsageReport {
  summary: { total_requests: number; total_cost: number; input_tokens: number; output_tokens: number; image_count: number; video_count: number }
  members: Array<{ user_id: number; email: string; username: string; total_requests: number; total_cost: number; input_tokens: number; output_tokens: number }>
  models: Array<{ model: string; total_requests: number; total_cost: number; total_tokens: number }>
}

export interface OrganizationAuditLog {
  id: number
  actor_user_id: number
  action: string
  target_type?: string
  target_id?: string
  detail: Record<string, unknown>
  created_at: string
}

export const organizationsAPI = {
  async list(): Promise<Organization[]> { return (await apiClient.get('/organizations')).data },
  async create(name: string, seatLimit: number): Promise<Organization> { return (await apiClient.post('/organizations', { name, seat_limit: seatLimit })).data },
  async delete(id: number): Promise<void> { await apiClient.delete('/organizations/delete', { params: { organization_id: id } }) },
  async get(id: number): Promise<Organization> { return (await apiClient.get('/organizations', { params: { organization_id: id } })).data },
  async members(id: number): Promise<OrganizationMember[]> { return (await apiClient.get('/organizations/members', { params: { organization_id: id } })).data },
  async updateMember(id: number, userId: number, input: { role?: string; status?: string; monthly_limit?: number }): Promise<void> { await apiClient.patch('/organizations/members', { organization_id: id, user_id: userId, ...input }) },
  async removeMember(id: number, userId: number): Promise<void> { await apiClient.delete('/organizations/members', { params: { organization_id: id, user_id: userId } }) },
  async invitations(id: number): Promise<OrganizationInvitation[]> { return (await apiClient.get('/organizations/invitations', { params: { organization_id: id } })).data },
  async invite(id: number, input: { email?: string; role: string; max_uses: number; expires_in_hours: number }): Promise<OrganizationInvitation> { return (await apiClient.post('/organizations/invitations', { organization_id: id, ...input })).data },
  async accept(token: string): Promise<Organization> { return (await apiClient.post('/organizations/invitations/accept', { token })).data },
  async fund(id: number, amount: number): Promise<Organization> { return (await apiClient.post('/organizations/fund', { organization_id: id, amount })).data },
  async usage(id: number): Promise<OrganizationUsageReport> { return (await apiClient.get('/organizations/usage', { params: { organization_id: id } })).data },
  async auditLogs(id: number): Promise<OrganizationAuditLog[]> { return (await apiClient.get('/organizations/audit-logs', { params: { organization_id: id } })).data },
}
