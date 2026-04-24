/**
 * Public Axios HTTP Client
 * Dedicated instance for /api/public/* endpoints.
 *
 * Differences from `./client.ts`:
 *   - No Authorization header (public endpoints must work unauthenticated).
 *   - No 401 refresh/redirect logic (public pages should never bounce to /login).
 *   - No ApiResponse unwrapping — status endpoints return raw DTOs.
 *
 * baseURL matches the main client so both respect VITE_API_BASE_URL. When the
 * default `/api/v1` is in effect we still hit `/api/public/...` paths correctly
 * because callers pass absolute `/api/public/...` URLs (axios treats a URL
 * starting with `/` as overriding the baseURL's path portion only when baseURL
 * itself is a full origin; for same-origin deployments we use an empty origin
 * so an absolute path hits the server root regardless of baseURL).
 */

import axios, { AxiosInstance } from 'axios'

// Public endpoints live at an absolute server path, not under /api/v1. Using
// an empty baseURL means callers supply the full `/api/public/...` path and it
// is sent as-is relative to the current origin.
const PUBLIC_API_BASE_URL = ''

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
