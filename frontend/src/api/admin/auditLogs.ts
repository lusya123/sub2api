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
  actor_role?: string
  module?: string
  action_type?: string
  exclude_action_type?: string
  exclude_successful_read?: boolean | string
  target_type?: string
  target_id?: string
  status_code?: number | string
  success?: boolean | string
  q?: string
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

export const auditLogsAPI = {
  list,
  getById
}

export default auditLogsAPI
