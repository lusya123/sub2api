import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import GlobeStage from '../GlobeStage.vue'

const state = vi.hoisted(() => ({
  webGLAvailable: true,
  probe: vi.fn(),
  route: { query: {} as Record<string, string> },
  replace: vi.fn(),
}))

vi.mock('@/utils/webgl', () => ({
  canCreateGlobeWebGLContext: () => {
    state.probe()
    return state.webGLAvailable
  },
}))

vi.mock('vue-router', () => ({
  useRoute: () => state.route,
  useRouter: () => ({ replace: state.replace }),
}))

vi.mock('../LiveGlobe.vue', async () => {
  const { defineComponent, h } = await import('vue')
  return {
    default: defineComponent({
      name: 'LiveGlobe',
      emits: ['webglUnavailable'],
      setup(_, { emit }) {
        return () => h('button', {
          'data-testid': 'live-globe',
          onClick: () => emit('webglUnavailable'),
        })
      },
    }),
  }
})

vi.mock('../LiveMap2D.vue', async () => {
  const { defineComponent, h } = await import('vue')
  return {
    default: defineComponent({
      name: 'LiveMap2D',
      setup() {
        return () => h('div', { 'data-testid': 'live-map-2d' })
      },
    }),
  }
})

function mountStage(extraProps: Record<string, unknown> = {}) {
  return mount(GlobeStage, {
    props: {
      snapshot: null,
      ...extraProps,
    },
  })
}

describe('GlobeStage WebGL fallback', () => {
  beforeEach(() => {
    localStorage.clear()
    state.webGLAvailable = true
    state.probe.mockReset()
    state.route.query = {}
    state.replace.mockReset()
  })

  it('uses the 2D map when the default 3D mode cannot create WebGL 2', () => {
    state.webGLAvailable = false

    const wrapper = mountStage()

    expect(wrapper.find('[data-testid="live-map-2d"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="live-globe"]').exists()).toBe(false)
    expect(wrapper.findAll('.toggle-btn')[0]?.attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('.toggle-btn')[1]?.classes()).toContain('active')
  })

  it('downgrades an explicit ?view=3d request when WebGL 2 is unavailable', () => {
    state.webGLAvailable = false
    state.route.query = { view: '3d' }

    const wrapper = mountStage({ syncToUrl: true })

    expect(wrapper.find('[data-testid="live-map-2d"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="live-globe"]').exists()).toBe(false)
  })

  it('does not probe WebGL when query, storage, or the explicit default selects 2D', () => {
    state.route.query = { view: '2d' }
    const queryWrapper = mountStage({ syncToUrl: true })
    expect(queryWrapper.find('[data-testid="live-map-2d"]').exists()).toBe(true)
    queryWrapper.unmount()

    state.route.query = {}
    localStorage.setItem('sub2api.globe.mode', '2d')
    const storageWrapper = mountStage()
    expect(storageWrapper.find('[data-testid="live-map-2d"]').exists()).toBe(true)
    storageWrapper.unmount()

    localStorage.clear()
    const defaultWrapper = mountStage({ defaultMode: '2d' })
    expect(defaultWrapper.find('[data-testid="live-map-2d"]').exists()).toBe(true)

    expect(state.probe).not.toHaveBeenCalled()
  })

  it('probes lazily when a user switches from an explicit 2D mode to 3D', async () => {
    state.webGLAvailable = false
    const wrapper = mountStage({ defaultMode: '2d' })
    expect(state.probe).not.toHaveBeenCalled()

    await wrapper.findAll('.toggle-btn')[0]!.trigger('click')

    expect(state.probe).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-testid="live-map-2d"]').exists()).toBe(true)
    expect(wrapper.findAll('.toggle-btn')[0]?.attributes('disabled')).toBeDefined()
  })

  it('keeps the default and explicit URL modes in 3D when WebGL 2 is available', () => {
    const defaultWrapper = mountStage()
    expect(defaultWrapper.find('[data-testid="live-globe"]').exists()).toBe(true)
    defaultWrapper.unmount()

    state.route.query = { view: '3d' }
    const urlWrapper = mountStage({ syncToUrl: true })
    expect(urlWrapper.find('[data-testid="live-globe"]').exists()).toBe(true)
    expect(urlWrapper.find('[data-testid="live-map-2d"]').exists()).toBe(false)
  })

  it('falls back if the real renderer loses context availability after preflight', async () => {
    const wrapper = mountStage()
    expect(wrapper.find('[data-testid="live-globe"]').exists()).toBe(true)

    await wrapper.get('[data-testid="live-globe"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="live-map-2d"]').exists()).toBe(true)
    expect(wrapper.findAll('.toggle-btn')[0]?.attributes('disabled')).toBeDefined()
  })
})
