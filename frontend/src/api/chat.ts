import { apiClient } from './client'

export interface ChatLaunchResponse {
  url: string
}

export interface LobeModelConfig {
  id: string
  display_name: string
}

export interface LobeProviderConfig {
  id: string
  display_name: string
  sdk_type: 'anthropic' | 'google' | 'openai' | string
  base_url: string
  models: LobeModelConfig[]
}

export interface LobeUserConfig {
  user_id: string
  providers: LobeProviderConfig[]
}

export const chatAPI = {
  async getLobeConfig(): Promise<LobeUserConfig> {
    const { data } = await apiClient.get<LobeUserConfig>('/chat/lobe-config')
    return data
  },

  async launch(): Promise<ChatLaunchResponse> {
    const { data } = await apiClient.post<ChatLaunchResponse>('/chat/launch', {}, { withCredentials: true })
    return data
  },

  async launchWithModel(provider: string, model: string): Promise<ChatLaunchResponse> {
    const { data } = await apiClient.post<ChatLaunchResponse>(
      '/chat/launch',
      {},
      {
        params: { provider, model },
        withCredentials: true
      }
    )
    return data
  }
}
