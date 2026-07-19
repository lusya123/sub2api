import { describe, expect, it } from 'vitest'

import { isOperatorAdminPath, isSuperAdminRoutePath } from '../adminAccess'

describe('security audit admin access', () => {
  it('keeps both audit consoles super-admin-only', () => {
    for (const path of ['/admin/audit-logs', '/admin/security-audit-logs']) {
      expect(isSuperAdminRoutePath(path)).toBe(true)
      expect(isOperatorAdminPath(path)).toBe(false)
    }
  })
})
