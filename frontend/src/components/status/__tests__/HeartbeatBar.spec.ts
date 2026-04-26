import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import HeartbeatBar from '../HeartbeatBar.vue'

describe('HeartbeatBar', () => {
  it('renders one beat per entry with the right color class', () => {
    const beats = [
      { ts: '2026-04-24T10:00:00Z', status: 'ok' as const },
      { ts: '2026-04-24T10:01:00Z', status: 'degraded' as const },
      { ts: '2026-04-24T10:02:00Z', status: 'down' as const },
      { ts: '2026-04-24T10:03:00Z', status: 'unknown' as const }
    ]
    const wrapper = mount(HeartbeatBar, { props: { beats } })
    const items = wrapper.findAll('[data-beat]')
    // 组件会补齐到 90,这里取最后 4 个验证颜色
    expect(items.length).toBeGreaterThanOrEqual(4)
    expect(items[items.length - 4].classes()).toContain('is-ok')
    expect(items[items.length - 3].classes()).toContain('is-degraded')
    expect(items[items.length - 2].classes()).toContain('is-down')
    expect(items[items.length - 1].classes()).toContain('is-unknown')
  })
})
