import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import EndpointDistributionChart from '../EndpointDistributionChart.vue'

const messages: Record<string, string> = {
  'usage.endpointDistribution': 'Endpoint Distribution',
  'usage.endpoint': 'Endpoint',
  'admin.dashboard.requests': 'Requests',
  'admin.dashboard.tokens': 'Tokens',
  'admin.dashboard.actual': 'Actual',
  'admin.dashboard.standard': 'Standard',
  'admin.dashboard.noDataAvailable': 'No data available',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('vue-chartjs', () => ({
  Doughnut: {
    props: ['data'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  },
}))

describe('EndpointDistributionChart', () => {
  it('hides standard cost for user endpoint stats that omit it', () => {
    const wrapper = mount(EndpointDistributionChart, {
      props: {
        endpointStats: [
          {
            endpoint: '/v1/messages',
            requests: 2,
            total_tokens: 123,
            actual_cost: 0.04,
          },
        ],
        showStandardCost: false,
        enableBreakdown: false,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    expect(wrapper.text()).toContain('/v1/messages')
    expect(wrapper.text()).toContain('$0.040')
    expect(wrapper.text()).not.toContain('Standard')
    expect(wrapper.findAll('thead th')).toHaveLength(4)
    expect(wrapper.findAll('tbody tr')[0].findAll('td')).toHaveLength(4)
  })
})
