import { apiClient } from './client'

export interface ChatLaunchResponse {
  url: string
}

export const chatAPI = {
  async launch(): Promise<ChatLaunchResponse> {
    const { data } = await apiClient.post<ChatLaunchResponse>('/chat/launch', {}, { withCredentials: true })
    return data
  }
}
