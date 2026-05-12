export function getPlatformTagClass(platform: string): string {
  switch (platform) {
    case 'anthropic': return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
    case 'openai': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
    case 'gemini': return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
    case 'antigravity': return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
    case 'minimax': return 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400'
    case 'zhipu': return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400'
    case 'zhipu_v4': return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400'
    case 'deepseek': return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400'
    default: return 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400'
  }
}

export function getPlatformTextClass(platform: string): string {
  switch (platform) {
    case 'anthropic': return 'text-orange-700 dark:text-orange-400'
    case 'openai': return 'text-emerald-700 dark:text-emerald-400'
    case 'gemini': return 'text-blue-700 dark:text-blue-400'
    case 'antigravity': return 'text-purple-700 dark:text-purple-400'
    case 'minimax': return 'text-pink-700 dark:text-pink-400'
    case 'zhipu': return 'text-cyan-700 dark:text-cyan-400'
    case 'zhipu_v4': return 'text-cyan-700 dark:text-cyan-400'
    case 'deepseek': return 'text-indigo-700 dark:text-indigo-400'
    default: return ''
  }
}
