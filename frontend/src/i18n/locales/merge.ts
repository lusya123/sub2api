export type LocaleMessages = Record<string, unknown>

function isPlainObject(value: unknown): value is LocaleMessages {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

/** Merge locally maintained message deltas into the upstream domain modules. */
export function mergeLocaleMessages(base: LocaleMessages, custom: LocaleMessages): LocaleMessages {
  const merged: LocaleMessages = { ...base }

  for (const [key, value] of Object.entries(custom)) {
    const current = merged[key]
    merged[key] = isPlainObject(current) && isPlainObject(value)
      ? mergeLocaleMessages(current, value)
      : value
  }

  return merged
}
