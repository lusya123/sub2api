import { apiClient } from '../client'
import type {
  ModelMarketplaceResponse,
  PublicStatusConfig,
  PublicStatusConfigAdminView
} from '@/types'

export async function list(): Promise<ModelMarketplaceResponse> {
  const { data } = await apiClient.get<ModelMarketplaceResponse>('/admin/model-marketplace')
  return data
}

export async function getStatusConfig(): Promise<PublicStatusConfigAdminView> {
  const { data } = await apiClient.get<PublicStatusConfigAdminView>('/admin/model-marketplace/status-config')
  return data
}

export async function updateStatusConfig(config: PublicStatusConfig): Promise<PublicStatusConfigAdminView> {
  const { data } = await apiClient.put<PublicStatusConfigAdminView>('/admin/model-marketplace/status-config', config)
  return data
}

export default {
  list,
  getStatusConfig,
  updateStatusConfig
}
