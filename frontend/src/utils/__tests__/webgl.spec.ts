import { describe, expect, it, vi } from 'vitest'

import {
  GLOBE_WEBGL_CONTEXT_OPTIONS,
  canCreateGlobeWebGLContext,
  isWebGLContextCreationError,
} from '../webgl'

function documentWithContext(getContext: ReturnType<typeof vi.fn>): Document {
  return {
    createElement: vi.fn(() => ({ getContext })),
  } as unknown as Document
}

describe('globe WebGL capability detection', () => {
  it('requires a WebGL 2 context using the same renderer attributes', () => {
    const loseContext = vi.fn()
    const getExtension = vi.fn(() => ({ loseContext }))
    const context = { getExtension } as unknown as WebGL2RenderingContext
    const getContext = vi.fn(() => context)

    expect(canCreateGlobeWebGLContext(documentWithContext(getContext))).toBe(true)
    expect(getContext).toHaveBeenCalledWith('webgl2', GLOBE_WEBGL_CONTEXT_OPTIONS)
    expect(getExtension).toHaveBeenCalledWith('WEBGL_lose_context')
    expect(loseContext).toHaveBeenCalledOnce()
  })

  it('fails closed when context creation returns null or throws', () => {
    expect(canCreateGlobeWebGLContext(documentWithContext(vi.fn(() => null)))).toBe(false)
    expect(canCreateGlobeWebGLContext(documentWithContext(vi.fn(() => {
      throw new DOMException('GPU unavailable')
    })))).toBe(false)
  })

  it('keeps a successful probe best-effort when releasing the context throws', () => {
    const context = {
      getExtension: vi.fn(() => {
        throw new DOMException('extension unavailable')
      }),
    } as unknown as WebGL2RenderingContext

    expect(canCreateGlobeWebGLContext(documentWithContext(vi.fn(() => context)))).toBe(true)
  })

  it('only classifies Three.js context-creation errors for fallback', () => {
    expect(isWebGLContextCreationError(new Error('Error creating WebGL context.'))).toBe(true)
    expect(isWebGLContextCreationError(
      new Error('Error creating WebGL context with your selected attributes.'),
    )).toBe(true)
    expect(isWebGLContextCreationError(new Error('three-globe import failed'))).toBe(false)
    expect(isWebGLContextCreationError('Error creating WebGL context.')).toBe(false)
  })
})
