/**
 * User-facing Model Marketplace API endpoints.
 * Read-only views for end users to inspect model marketplace availability/status.
 */

import { apiClient } from './client'
import type { Provider, MonitorStatus } from './admin/modelMarketplaceMonitor'
import type { UserSupportedModelPricing } from './channels'

export type { Provider, MonitorStatus } from './admin/modelMarketplaceMonitor'

export interface UserModelMarketplaceExtraModel {
  model: string
  status: MonitorStatus
  latency_ms: number | null
  ping_latency_ms: number | null
  availability_7d: number
  pricing: UserSupportedModelPricing | null
}

export interface ModelMarketplaceTimelinePoint {
  status: MonitorStatus
  latency_ms: number | null
  ping_latency_ms: number | null
  checked_at: string
}

export interface UserModelMarketplaceView {
  id: number
  name: string
  provider: Provider
  group_name: string
  primary_model: string
  primary_status: MonitorStatus
  primary_latency_ms: number | null
  primary_ping_latency_ms: number | null
  availability_7d: number
  primary_pricing: UserSupportedModelPricing | null
  extra_models: UserModelMarketplaceExtraModel[]
  timeline: ModelMarketplaceTimelinePoint[]
}

export interface UserModelMarketplaceListResponse {
  items: UserModelMarketplaceView[]
}

export interface UserModelMarketplaceModelDetail {
  model: string
  latest_status: MonitorStatus
  latest_latency_ms: number | null
  availability_7d: number
  availability_15d: number
  availability_30d: number
  avg_latency_7d_ms: number | null
  pricing: UserSupportedModelPricing | null
}

export interface UserModelMarketplaceDetail {
  id: number
  name: string
  provider: Provider
  group_name: string
  models: UserModelMarketplaceModelDetail[]
}

export async function list(options?: { signal?: AbortSignal }): Promise<UserModelMarketplaceListResponse> {
  const { data } = await apiClient.get<UserModelMarketplaceListResponse>('/model-marketplace', {
    signal: options?.signal,
  })
  return data
}

export async function status(id: number): Promise<UserModelMarketplaceDetail> {
  const { data } = await apiClient.get<UserModelMarketplaceDetail>(`/model-marketplace/${id}/status`)
  return data
}

export const modelMarketplaceUserAPI = {
  list,
  status,
}

export default modelMarketplaceUserAPI
