import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post }
}))

import { auditAPI } from '@/api/admin/audit'
import { auditLogsAPI } from '@/api/admin/auditLogs'

describe('admin audit API route contracts', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('keeps upstream management audit on /admin/audit-logs', async () => {
    await auditLogsAPI.list({ page: 2 })
    await auditLogsAPI.getById(12)
    await auditLogsAPI.getBalanceSummary({ actor_email: 'admin@example.test' })

    expect(get).toHaveBeenNthCalledWith(1, '/admin/audit-logs', { params: { page: 2 } })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/audit-logs/12')
    expect(get).toHaveBeenNthCalledWith(3, '/admin/audit-logs/balance-summary', {
      params: { actor_email: 'admin@example.test' }
    })
  })

  it('keeps local security audit on /admin/security-audit-logs', async () => {
    await auditAPI.list({ page: 3 })
    await auditAPI.get(21)
    await auditAPI.clear('123456')

    expect(get).toHaveBeenNthCalledWith(1, '/admin/security-audit-logs', { params: { page: 3 } })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/security-audit-logs/21')
    expect(post).toHaveBeenCalledWith('/admin/security-audit-logs/clear', { totp_code: '123456' })
  })
})
