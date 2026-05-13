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
  display_name_zh: string
  display_name_en: string
  call_model: string
  request_url: string
  status: MonitorStatus
  latency_ms: number | null
  ping_latency_ms: number | null
  availability_7d: number
  pricing: UserSupportedModelPricing | null
  timeline: ModelMarketplaceTimelinePoint[]
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
  effective_rate: number
  primary_model: string
  primary_display_name_zh: string
  primary_display_name_en: string
  primary_call_model: string
  primary_request_url: string
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

export interface ModelMarketplaceExchangeRate {
  base: string
  quote: string
  rate: number
  source: string
  updated_at: string
  fallback: boolean
}

export interface UserModelMarketplaceModelDetail {
  model: string
  display_name_zh: string
  display_name_en: string
  call_model: string
  request_url: string
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
  effective_rate: number
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

export async function exchangeRate(): Promise<ModelMarketplaceExchangeRate> {
  const { data } = await apiClient.get<ModelMarketplaceExchangeRate>('/model-marketplace/exchange-rate')
  return data
}

export const modelMarketplaceUserAPI = {
  list,
  status,
  exchangeRate,
}

export default modelMarketplaceUserAPI
