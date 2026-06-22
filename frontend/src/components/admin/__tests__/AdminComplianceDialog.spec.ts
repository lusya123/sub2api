import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AdminComplianceDialog from '../AdminComplianceDialog.vue'

const {
  authStore,
  complianceStore,
  appStore,
} = vi.hoisted(() => ({
  authStore: {
    isAuthenticated: true,
    isAdmin: false,
    canAccessAdmin: true,
    logout: vi.fn(),
  },
  complianceStore: {
    status: {
      required: true,
      version: 'v2026.06.10',
      document_url_zh: 'https://example.test/admin-compliance.zh.md',
      document_url_en: 'https://example.test/admin-compliance.en.md',
    },
    shouldShow: true,
    expectedPhrase: '我已阅读、理解并同意 Sub2API 部署与运营合规承诺',
    submitting: false,
    accept: vi.fn(),
  },
  appStore: {
    showSuccess: vi.fn(),
    showError: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => authStore,
  useAdminComplianceStore: () => complianceStore,
  useAppStore: () => appStore,
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh',
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'adminCompliance.title': '部署与运营合规确认',
    'adminCompliance.version': '版本',
    'adminCompliance.openDocument': '在 GitHub 查看协议文件',
    'adminCompliance.documentSource': '协议正文来自本项目仓库中的 Markdown 文件。',
    'adminCompliance.blockingNotice': '继续使用控制台前，须完成部署与运营合规确认。',
    'adminCompliance.riskNotice': '本确认用于提示自部署实例的合规义务与运营风险。',
    'adminCompliance.inputLabel': '请输入确认短语',
    'adminCompliance.inputPlaceholder': '输入确认短语以继续',
    'adminCompliance.legalNote': '确认后系统将记录必要留痕信息。',
    'adminCompliance.logout': '退出登录',
    'adminCompliance.accept': '确认并继续',
    'adminCompliance.inputMismatch': '确认短语不匹配',
    'adminCompliance.accepted': '合规确认已记录',
    'adminCompliance.acceptFailed': '合规确认失败',
    'common.submitting': '提交中',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] || key,
    }),
  }
})

vi.mock('marked', () => ({
  marked: {
    setOptions: vi.fn(),
    parse: (value: string) => value,
  },
}))

vi.mock('dompurify', () => ({
  default: {
    sanitize: (value: string) => value,
  },
}))

const mountDialog = () => mount(AdminComplianceDialog, {
  global: {
    stubs: {
      BaseDialog: {
        props: ['show', 'title'],
        template: '<div v-if="show" data-test="dialog"><h1>{{ title }}</h1><slot /><slot name="footer" /></div>',
      },
      Icon: true,
      Input: {
        props: ['id', 'modelValue', 'placeholder', 'disabled', 'error'],
        emits: ['update:modelValue', 'enter'],
        template: `
          <div>
            <input
              :id="id"
              :value="modelValue"
              :placeholder="placeholder"
              :disabled="disabled"
              @input="$emit('update:modelValue', $event.target.value)"
              @keyup.enter="$emit('enter')"
            />
            <span v-if="error" data-test="input-error">{{ error }}</span>
          </div>
        `,
      },
    },
  },
})

describe('AdminComplianceDialog', () => {
  beforeEach(() => {
    authStore.isAuthenticated = true
    authStore.isAdmin = false
    authStore.canAccessAdmin = true
    complianceStore.shouldShow = true
    complianceStore.submitting = false
    vi.clearAllMocks()
  })

  it('shows the acknowledgement dialog for operator admins', () => {
    const wrapper = mountDialog()

    expect(wrapper.find('[data-test="dialog"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('部署与运营合规确认')
    expect(wrapper.find('#admin-compliance-phrase').exists()).toBe(true)
  })

  it('does not show the acknowledgement dialog for regular users', () => {
    authStore.canAccessAdmin = false

    const wrapper = mountDialog()

    expect(wrapper.find('[data-test="dialog"]').exists()).toBe(false)
  })
})
