import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface AdminAuditLog {
  id: number
  created_at: string
  actor_user_id?: number | null
  actor_email?: string | null
  actor_role: string
  method: string
  route_template: string
  path: string
  module: string
  action: string
  action_type: string
  target_type?: string | null
  target_id?: number | string | null
  user_refs?: Record<string, string> | null
  status_code: number
  success: boolean
  error_code?: string | null
  error_message?: string | null
  ip_address?: string | null
  user_agent?: string | null
  summary?: string | null
  query_params?: Record<string, unknown> | null
  request_body?: unknown
  duration_ms: number
}

export interface AdminAuditLogFilters {
  page?: number
  page_size?: number
  start_time?: string
  end_time?: string
  actor_user_id?: number | string
  actor_email?: string
  actor_role?: string
  module?: string
  action_type?: string
  exclude_action_type?: string
  exclude_successful_read?: boolean | string
  target_type?: string
  target_id?: string
  target_email?: string
  status_code?: number | string
  success?: boolean | string
  q?: string
}

export interface AdminAuditBalanceSummaryItem {
  actor_user_id: number
  actor_email: string
  actor_role: string
  add_amount: number
  subtract_amount: number
  net_amount: number
  add_count: number
  subtract_count: number
  set_count: number
  total_count: number
  target_user_count: number
  first_at: string
  last_at: string
}

export interface AdminAuditBalanceSummaryTotals {
  actor_count: number
  add_amount: number
  subtract_amount: number
  net_amount: number
  add_count: number
  subtract_count: number
  set_count: number
  total_count: number
  target_user_count: number
}

export interface AdminAuditBalanceSummary {
  items: AdminAuditBalanceSummaryItem[]
  totals: AdminAuditBalanceSummaryTotals
}

export async function list(filters: AdminAuditLogFilters = {}): Promise<PaginatedResponse<AdminAuditLog>> {
  const params = Object.fromEntries(
    Object.entries(filters).filter(([, value]) => value !== '' && value !== undefined && value !== null)
  )
  const { data } = await apiClient.get<PaginatedResponse<AdminAuditLog>>('/admin/audit-logs', { params })
  return data
}

export async function getById(id: number): Promise<AdminAuditLog> {
  const { data } = await apiClient.get<AdminAuditLog>(`/admin/audit-logs/${id}`)
  return data
}

export async function getBalanceSummary(filters: AdminAuditLogFilters = {}): Promise<AdminAuditBalanceSummary> {
  const params = Object.fromEntries(
    Object.entries(filters).filter(([, value]) => value !== '' && value !== undefined && value !== null)
  )
  const { data } = await apiClient.get<AdminAuditBalanceSummary>('/admin/audit-logs/balance-summary', { params })
  return data
}

export const auditLogsAPI = {
  list,
  getById,
  getBalanceSummary
}

export default auditLogsAPI
