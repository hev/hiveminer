<template>
  <div class="p-3">
    <h2 class="text-sm font-semibold text-gray-500 uppercase mb-3">Extracted Entries</h2>

    <div v-if="!threadState" class="text-gray-400 text-sm text-center py-8">
      Select a thread
    </div>

    <div v-else-if="threadState.status !== 'extracted' && threadState.status !== 'ranked'" class="text-gray-400 text-sm text-center py-8">
      <p>Not extracted yet</p>
      <p class="mt-2">Status: {{ threadState.status }}</p>
    </div>

    <div v-else class="space-y-6">
      <div
        v-for="(entry, idx) in entries"
        :key="idx"
        class="border rounded-lg overflow-hidden"
      >
        <div class="bg-gray-50 px-3 py-2 border-b text-xs font-semibold text-gray-500 uppercase flex items-center gap-2">
          <span>Entry {{ idx + 1 }}</span>
          <span v-if="entry.rank_score != null" class="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-bold" :class="rankScoreClass(entry.rank_score)">
            {{ Math.round(entry.rank_score) }}pts
          </span>
          <span
            v-for="flag in (entry.rank_flags || [])"
            :key="flag"
            class="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium"
            :class="flagClass(flag)"
          >
            {{ flag }}
          </span>
          <span v-if="primaryValue(entry)" class="text-gray-700 normal-case ml-1">— {{ primaryValue(entry) }}</span>
        </div>
        <div class="p-3 space-y-4">
          <FieldValue
            v-for="field in entry.fields"
            :key="field.id"
            :field="field"
            :thread-permalink="threadState.permalink"
          />
          <!-- Entry-level deduped source links -->
          <div v-if="entryLinks(entry).length" class="pt-2 border-t">
            <div class="text-xs text-gray-500 uppercase mb-1">Sources</div>
            <div class="flex flex-wrap gap-2">
              <a
                v-for="link in entryLinks(entry)"
                :key="link.url"
                :href="link.url"
                target="_blank"
                class="inline-flex items-center gap-1 text-xs text-orange-600 hover:text-orange-800 hover:underline bg-orange-50 px-2 py-1 rounded"
              >
                u/{{ link.author }}
                <svg class="w-3 h-3 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/></svg>
              </a>
            </div>
          </div>
        </div>
      </div>

      <div v-if="!entries.length" class="text-gray-400 text-sm text-center py-4">
        No entries extracted
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useThreadStore } from '../stores/threads'
import FieldValue from './FieldValue.vue'

const store = useThreadStore()

const threadState = computed(() => store.currentThreadState)

const entries = computed(() => {
  const ts = threadState.value
  if (!ts) return []
  // Support new entries format and legacy fields format
  if (ts.entries?.length) return ts.entries
  if (ts.fields?.length) return [{ fields: ts.fields }]
  return []
})

function primaryValue(entry) {
  if (!entry.fields?.length) return null
  const first = entry.fields[0]
  if (first.value == null) return null
  if (typeof first.value === 'string') return first.value
  return null
}

function entryLinks(entry) {
  // Use pre-computed entry.links if available, otherwise derive from evidence
  const permalink = threadState.value?.permalink || ''
  const seen = new Map()

  if (entry.links?.length) {
    // Links from backend (permalink paths)
    for (const link of entry.links) {
      if (!seen.has(link)) {
        // Find the author from evidence
        const author = findAuthorForLink(entry, link)
        seen.set(link, { url: `https://www.reddit.com${link}`, author })
      }
    }
  } else {
    // Fallback: derive from evidence comment_ids
    for (const field of entry.fields || []) {
      for (const ev of field.evidence || []) {
        if (ev.comment_id && ev.comment_id !== 'post_content' && permalink) {
          const p = permalink.endsWith('/') ? permalink : permalink + '/'
          const link = p + ev.comment_id + '/'
          if (!seen.has(link)) {
            seen.set(link, { url: `https://www.reddit.com${link}`, author: ev.author?.replace(/^u\//, '') || '' })
          }
        }
      }
    }
  }

  return [...seen.values()]
}

function rankScoreClass(score) {
  if (score >= 70) return 'bg-green-100 text-green-800'
  if (score >= 40) return 'bg-yellow-100 text-yellow-800'
  return 'bg-red-100 text-red-800'
}

function flagClass(flag) {
  switch (flag) {
    case 'spam':
    case 'off_topic':
      return 'bg-red-100 text-red-700'
    case 'joke':
    case 'outdated':
      return 'bg-orange-100 text-orange-700'
    case 'duplicate':
    case 'low_effort':
      return 'bg-yellow-100 text-yellow-700'
    default:
      return 'bg-gray-100 text-gray-600'
  }
}

function findAuthorForLink(entry, link) {
  // Extract comment_id from link path and match against evidence
  const parts = link.replace(/\/$/, '').split('/')
  const commentId = parts[parts.length - 1]
  for (const field of entry.fields || []) {
    for (const ev of field.evidence || []) {
      if (ev.comment_id === commentId) {
        return ev.author?.replace(/^u\//, '') || ''
      }
    }
  }
  return ''
}
</script>
