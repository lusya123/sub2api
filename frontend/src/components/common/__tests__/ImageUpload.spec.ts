import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ImageUpload from '../ImageUpload.vue'

function setInputFile(input: HTMLInputElement, file: File) {
  Object.defineProperty(input, 'files', {
    configurable: true,
    value: [file],
  })
}

async function upload(wrapper: ReturnType<typeof mount>, file: File) {
  const input = wrapper.get('input[type="file"]')
  setInputFile(input.element as HTMLInputElement, file)
  await input.trigger('change')
  await flushPromises()
}

describe('ImageUpload', () => {
  beforeEach(() => {
    class MockFileReader {
      result: string | ArrayBuffer | null = null
      onload: ((event: ProgressEvent<FileReader>) => void) | null = null
      onerror: (() => void) | null = null

      readAsDataURL(file: File) {
        this.result = `data:${file.type};base64,bW9jaw==`
        this.onload?.({ target: this } as unknown as ProgressEvent<FileReader>)
      }

      readAsText(_file: File) {
        this.result = 'mock-svg'
        this.onload?.({ target: this } as unknown as ProgressEvent<FileReader>)
      }
    }

    vi.stubGlobal('FileReader', MockFileReader)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('emits a data URL for a valid image file', async () => {
    const wrapper = mount(ImageUpload, {
      props: {
        modelValue: '',
        mode: 'image',
        maxSize: 500 * 1024,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const file = new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], 'qr.png', {
      type: 'image/png',
    })

    await upload(wrapper, file)

    const emitted = wrapper.emitted('update:modelValue')?.[0]?.[0]
    expect(emitted).toEqual(expect.stringContaining('data:image/png;base64,'))
    expect(wrapper.text()).not.toContain('File too large')
  })

  it('rejects image files larger than the configured maximum', async () => {
    const wrapper = mount(ImageUpload, {
      props: {
        modelValue: '',
        mode: 'image',
        maxSize: 500 * 1024,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const file = new File([new Uint8Array(501 * 1024)], 'too-large.png', {
      type: 'image/png',
    })

    await upload(wrapper, file)

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.text()).toContain('File too large')
    expect(wrapper.text()).toContain('max 500 KB')
  })

  it('rejects non-image files in image mode', async () => {
    const wrapper = mount(ImageUpload, {
      props: {
        modelValue: '',
        mode: 'image',
        maxSize: 500 * 1024,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const file = new File(['not an image'], 'note.txt', {
      type: 'text/plain',
    })

    await upload(wrapper, file)

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.text()).toContain('Please select an image file')
  })

  it('emits an empty value when removing an existing image', async () => {
    const wrapper = mount(ImageUpload, {
      props: {
        modelValue: 'data:image/png;base64,abc',
        mode: 'image',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([''])
  })
})
