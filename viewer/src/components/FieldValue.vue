<template>
  <div class="bg-white rounded-lg p-3 border">
    <!-- Field Header -->
    <div class="flex items-center justify-between mb-2">
      <span class="font-medium text-gray-900 text-sm">{{ field.id }}</span>
      <ConfidenceBadge :confidence="field.confidence" />
    </div>

    <!-- Value -->
    <div class="text-gray-700 text-sm mb-2">
      <template v-if="field.value === null">
        <span class="text-gray-400 italic">Not found</span>
      </template>
      <template v-else-if="Array.isArray(field.value)">
        <ul class="list-disc list-inside space-y-1">
          <li v-for="(item, i) in field.value" :key="i">{{ item }}</li>
        </ul>
      </template>
      <template v-else>
        {{ field.value }}
      </template>
    </div>

    <!-- Field Links -->
    <div v-if="fieldLinks.length" class="flex flex-wrap gap-1.5 mt-1">
      <a
        v-for="link in fieldLinks"
        :key="link.url"
        :href="link.url"
        target="_blank"
        class="inline-flex items-center gap-0.5 text-xs text-orange-600 hover:text-orange-800 hover:underline"
      >
        u/{{ link.author }}
        <svg class="w-2.5 h-2.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/></svg>
      </a>
    </div>

    <!-- Evidence -->
    <div v-if="field.evidence?.length" class="mt-2 pt-2 border-t">
      <div class="text-xs text-gray-500 uppercase mb-1">Evidence</div>
      <div class="space-y-1">
        <div
          v-for="(ev, i) in field.evidence"
          :key="i"
          class="text-xs text-gray-600 bg-gray-50 p-2 rounded"
        >
          <span v-if="ev.author" class="font-medium">u/{{ ev.author }}: </span>
          "{{ truncate(ev.text, 150) }}"
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import ConfidenceBadge from './ConfidenceBadge.vue'

const props = defineProps({
  field: {
    type: Object,
    required: true
  },
  threadPermalink: {
    type: String,
    default: ''
  }
})

const fieldLinks = computed(() => {
  const base = 'https://www.reddit.com'
  const seen = new Map()

  // Use pre-computed links array if available
  if (props.field.links?.length) {
    for (const link of props.field.links) {
      if (!seen.has(link)) {
        const author = findAuthorForLink(link)
        seen.set(link, { url: `${base}${link}`, author })
      }
    }
  } else if (props.threadPermalink) {
    // Fallback: derive from evidence
    const p = props.threadPermalink.endsWith('/') ? props.threadPermalink : props.threadPermalink + '/'
    for (const ev of props.field.evidence || []) {
      if (ev.comment_id && ev.comment_id !== 'post_content') {
        const link = p + ev.comment_id + '/'
        if (!seen.has(link)) {
          seen.set(link, { url: `${base}${link}`, author: ev.author?.replace(/^u\//, '') || '' })
        }
      }
    }
  }

  return [...seen.values()]
})

function findAuthorForLink(link) {
  const parts = link.replace(/\/$/, '').split('/')
  const commentId = parts[parts.length - 1]
  for (const ev of props.field.evidence || []) {
    if (ev.comment_id === commentId) {
      return ev.author?.replace(/^u\//, '') || ''
    }
  }
  return ''
}

function truncate(text, length) {
  if (!text) return ''
  if (text.length <= length) return text
  return text.slice(0, length) + '...'
}
</script>
