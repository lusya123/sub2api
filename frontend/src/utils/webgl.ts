const WEBGL_CONTEXT_CREATION_MESSAGES = new Set([
  'Error creating WebGL context.',
  'Error creating WebGL context with your selected attributes.',
])

/**
 * Match the context options used by LiveGlobe's WebGLRenderer. Three.js r184
 * requires WebGL 2, so a WebGL 1 context is not sufficient for the 3D view.
 */
export const GLOBE_WEBGL_CONTEXT_OPTIONS = {
  alpha: true,
  antialias: true,
  depth: true,
  failIfMajorPerformanceCaveat: false,
  premultipliedAlpha: true,
  preserveDrawingBuffer: false,
  powerPreference: 'high-performance',
  stencil: false,
} as const satisfies WebGLContextAttributes

/**
 * Probe with a detached canvas before mounting Three.js. Some browsers expose
 * the WebGL APIs but cannot create a context (notably headless/disabled-GPU
 * environments), which would otherwise make WebGLRenderer throw on mount.
 */
export function canCreateGlobeWebGLContext(targetDocument?: Document): boolean {
  const doc = targetDocument ?? (typeof document === 'undefined' ? null : document)
  if (!doc) return false

  let context: WebGL2RenderingContext | null = null
  try {
    const canvas = doc.createElement('canvas')
    context = canvas.getContext(
      'webgl2',
      GLOBE_WEBGL_CONTEXT_OPTIONS,
    ) as WebGL2RenderingContext | null
    return context !== null
  } catch {
    // A failed capability probe means this environment cannot safely render
    // the globe. No application error is being handled here.
    return false
  } finally {
    // The probe canvas is temporary. Explicitly release its context instead of
    // waiting for GC so it does not consume one of the browser's limited WebGL
    // slots before Three.js creates the real renderer.
    try {
      context?.getExtension('WEBGL_lose_context')?.loseContext()
    } catch {
      // Capability detection must remain best-effort even in partial/mocked
      // WebGL implementations.
    }
  }
}

/** Only classify the two context-creation errors emitted by Three.js. */
export function isWebGLContextCreationError(error: unknown): error is Error {
  return error instanceof Error && WEBGL_CONTEXT_CREATION_MESSAGES.has(error.message)
}
