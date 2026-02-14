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
import ConfidenceBadge from './ConfidenceBadge.vue'

defineProps({
  field: {
    type: Object,
    required: true
  }
})

function truncate(text, length) {
  if (!text) return ''
  if (text.length <= length) return text
  return text.slice(0, length) + '...'
}
</script>
