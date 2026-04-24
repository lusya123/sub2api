<template>
  <article
    class="rounded-xl border border-neutral-800 bg-neutral-950/80 p-5 text-neutral-100"
  >
    <!-- 头部 -->
    <header class="flex items-start gap-3">
      <!-- 8 尖星图标 -->
      <svg
        class="h-6 w-6 text-orange-400 flex-shrink-0 mt-0.5"
        viewBox="0 0 24 24"
        fill="currentColor"
        aria-hidden="true"
      >
        <path
          d="M12 2 L13.5 8.5 L19.5 5.5 L16.5 11.5 L22 12 L16.5 12.5 L19.5 18.5 L13.5 15.5 L12 22 L10.5 15.5 L4.5 18.5 L7.5 12.5 L2 12 L7.5 11.5 L4.5 5.5 L10.5 8.5 Z"
        />
      </svg>
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2 flex-wrap">
          <h3 class="text-base font-bold text-neutral-100 truncate">
            {{ model.name }}
          </h3>
          <button
            type="button"
            class="inline-flex items-center justify-center h-5 w-5 rounded text-neutral-500 hover:text-neutral-200 hover:bg-neutral-800 transition"
            :title="copied ? '已复制' : '复制模型名'"
            @click="copyName"
          >
            <svg
              v-if="!copied"
              class="h-3.5 w-3.5"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
            </svg>
            <svg
              v-else
              class="h-3.5 w-3.5 text-green-400"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <polyline points="20 6 9 17 4 12" />
            </svg>
          </button>
          <span v-if="copied" class="text-[10px] text-green-400">已复制</span>
        </div>
        <div class="mt-0.5 flex items-center gap-2 text-[10px] text-neutral-500">
          <span class="uppercase tracking-widest">{{ model.provider }}</span>
          <span v-if="model.release_date" class="text-neutral-600">·</span>
          <span v-if="model.release_date">{{ model.release_date }}</span>
        </div>
      </div>
    </header>

    <!-- Prompt Caching 徽章 -->
    <div v-if="model.prompt_caching" class="mt-3">
      <span
        class="inline-flex items-center gap-1 rounded-full bg-green-500/15 text-green-400 px-2.5 py-0.5 text-[11px] font-medium"
      >
        <span aria-hidden="true">⚡</span>
        Prompt Caching
      </span>
    </div>

    <!-- 备注框 -->
    <div
      v-if="model.note"
      class="mt-3 rounded-lg bg-neutral-900 border border-neutral-800 px-3 py-2 text-xs text-neutral-400"
    >
      {{ model.note }}
    </div>

    <!-- 定价 2x2 -->
    <div class="mt-4 grid grid-cols-2 gap-x-4 gap-y-2">
      <div class="flex items-center justify-between">
        <span class="text-xs text-neutral-500">
          <span aria-hidden="true" class="mr-1">↓</span>输入
        </span>
        <span class="text-sm font-semibold text-neutral-100">
          ${{ formatPrice(model.pricing.input_per_mtok) }}
        </span>
      </div>
      <div class="flex items-center justify-between">
        <span class="text-xs text-neutral-500">
          <span aria-hidden="true" class="mr-1">↑</span>输出
        </span>
        <span class="text-sm font-semibold text-neutral-100">
          ${{ formatPrice(model.pricing.output_per_mtok) }}
        </span>
      </div>
      <div class="flex items-center justify-between">
        <span class="text-xs text-neutral-500">
          <span aria-hidden="true" class="mr-1">📝</span>缓存写入
        </span>
        <span class="text-sm font-semibold text-neutral-100">
          ${{ formatPrice(model.pricing.cache_write) }}
        </span>
      </div>
      <div class="flex items-center justify-between">
        <span class="text-xs text-neutral-500">
          <span aria-hidden="true" class="mr-1">📖</span>缓存读取
        </span>
        <span class="text-sm font-semibold text-neutral-100">
          ${{ formatPrice(model.pricing.cache_read) }}
        </span>
      </div>
    </div>
    <div class="mt-1 text-right text-[10px] uppercase tracking-wide text-neutral-600">
      USD 每百万 TOKENS
    </div>

    <!-- 总可用率 + 心跳条 -->
    <div class="mt-5">
      <div class="flex items-center gap-2">
        <span
          class="inline-block h-2 w-2 rounded-full"
          :class="availabilityDotClass"
          aria-hidden="true"
        />
        <span class="text-2xl font-bold text-neutral-100 font-mono tracking-tight">
          {{ model.availability_pct.toFixed(2) }}%
        </span>
        <span class="text-xs text-neutral-500 ml-1">可用率</span>
      </div>
      <div class="mt-2">
        <HeartbeatBar :beats="model.heartbeats || []" />
      </div>
      <div class="mt-1 flex items-center justify-between text-[10px] text-neutral-500">
        <span>90 min ago</span>
        <span>现在</span>
      </div>
    </div>

    <!-- 展开/收起 -->
    <div class="mt-4 border-t border-neutral-800 pt-3">
      <button
        type="button"
        class="w-full flex items-center gap-1 text-xs text-neutral-400 hover:text-neutral-200 transition"
        @click="expanded = !expanded"
      >
        <span aria-hidden="true">{{ expanded ? '⌄' : '⌃' }}</span>
        <span>{{ channelCount }} 个渠道</span>
      </button>
      <div v-if="expanded" class="mt-2 space-y-3">
        <GroupSection
          v-for="(g, idx) in model.groups"
          :key="`${g.name}-${idx}`"
          :group="g"
        />
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { StatusModel } from '@/types'
import HeartbeatBar from './HeartbeatBar.vue'
import GroupSection from './GroupSection.vue'

interface Props {
  model: StatusModel
}

const props = defineProps<Props>()

const expanded = ref(true)
const copied = ref(false)

const channelCount = computed(() => {
  const groups = props.model.groups ?? []
  return groups.reduce((sum, g) => sum + (g.channels?.length ?? 0), 0)
})

const availabilityDotClass = computed(() => {
  const pct = props.model.availability_pct ?? 0
  if (pct >= 99) return 'bg-green-500'
  if (pct >= 90) return 'bg-orange-500'
  return 'bg-red-500'
})

function formatPrice(v: number): string {
  if (v == null || Number.isNaN(v)) return '0'
  // 去掉多余的小数 0：5 -> "5"，0.5 -> "0.5"，6.25 -> "6.25"
  return Number(v)
    .toFixed(4)
    .replace(/\.?0+$/, '')
}

async function copyName() {
  const name = props.model.name
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(name)
    } else {
      // fallback
      const ta = document.createElement('textarea')
      ta.value = name
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    copied.value = false
  }
}
</script>
