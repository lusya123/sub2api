/**
 * Public Axios HTTP Client
 * Dedicated instance for /api/public/* endpoints.
 *
 * Differences from `./client.ts`:
 *   - No Authorization header (public endpoints must work unauthenticated).
 *   - No 401 refresh/redirect logic (public pages should never bounce to /login).
 *   - No ApiResponse unwrapping — status endpoints return raw DTOs.
 *
 * VITE_PUBLIC_API_BASE_URL can point the public status page at another origin
 * such as the production cloud server. The default empty baseURL keeps same-
 * origin deployments hitting `/api/public/...` on the current host.
 */

import axios, { AxiosInstance } from 'axios'

const PUBLIC_API_BASE_URL = import.meta.env.VITE_PUBLIC_API_BASE_URL || ''

export const publicClient: AxiosInstance = axios.create({
  baseURL: PUBLIC_API_BASE_URL,
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// Minimal response error normalization — never redirect, never refresh tokens.
publicClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error?.response) {
      const { status, data } = error.response
      const apiData = (typeof data === 'object' && data !== null ? data : {}) as Record<string, any>
      return Promise.reject({
        status,
        code: apiData.code,
        message: apiData.message || apiData.detail || error.message
      })
    }
    return Promise.reject({
      status: 0,
      message: 'Network error. Please check your connection.'
    })
  }
)

export default publicClient
