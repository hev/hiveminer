<template>
  <div class="p-3">
    <h2 class="text-sm font-semibold text-gray-500 uppercase mb-3">Extracted Entries</h2>

    <div v-if="!threadState" class="text-gray-400 text-sm text-center py-8">
      Select a thread
    </div>

    <div v-else-if="threadState.status !== 'extracted'" class="text-gray-400 text-sm text-center py-8">
      <p>Not extracted yet</p>
      <p class="mt-2">Status: {{ threadState.status }}</p>
    </div>

    <div v-else class="space-y-6">
      <div
        v-for="(entry, idx) in entries"
        :key="idx"
        class="border rounded-lg overflow-hidden"
      >
        <div class="bg-gray-50 px-3 py-2 border-b text-xs font-semibold text-gray-500 uppercase">
          Entry {{ idx + 1 }}
          <span v-if="primaryValue(entry)" class="text-gray-700 normal-case ml-1">— {{ primaryValue(entry) }}</span>
        </div>
        <div class="p-3 space-y-4">
          <FieldValue
            v-for="field in entry.fields"
            :key="field.id"
            :field="field"
            :thread-permalink="threadState.permalink"
          />
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
</script>
