/**
 * Public Status Page API client.
 * Backed by the unauthenticated /api/public/status/* endpoints.
 */

import { publicClient } from './publicClient'
import type { StatusModel } from '@/types'

export interface PublicStatusListResponse {
  models: StatusModel[]
}

export const publicStatusApi = {
  /**
   * GET /api/public/status/models
   * Lightweight listing — groups/heartbeats are NOT populated.
   */
  listModels: () =>
    publicClient.get<PublicStatusListResponse>('/api/public/status/models'),

  /**
   * GET /api/public/status/model/:name
   * Full detail — groups + per-channel heartbeats populated.
   */
  getModel: (name: string) =>
    publicClient.get<StatusModel>(
      `/api/public/status/model/${encodeURIComponent(name)}`
    )
}

export default publicStatusApi
