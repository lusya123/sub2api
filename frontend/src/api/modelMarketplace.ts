import { apiClient } from './client'
import type { ModelMarketplaceResponse } from '@/types'

export async function list(): Promise<ModelMarketplaceResponse> {
  const { data } = await apiClient.get<ModelMarketplaceResponse>('/model-marketplace')
  return data
}

export const modelMarketplaceAPI = {
  list
}

export default modelMarketplaceAPI
