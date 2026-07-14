import { describe, expect, it, vi } from 'vitest'
import { navigateInCurrentTab } from '../navigation'

describe('navigateInCurrentTab', () => {
  it('uses a top-level location assignment for the target URL', () => {
    const assign = vi.fn()

    navigateInCurrentTab('https://chat.example.com/signin', { assign })

    expect(assign).toHaveBeenCalledOnce()
    expect(assign).toHaveBeenCalledWith('https://chat.example.com/signin')
  })
})
