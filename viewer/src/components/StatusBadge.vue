<template>
  <span :class="badgeClasses">
    {{ label }}
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: {
    type: String,
    required: true
  }
})

const badgeClasses = computed(() => {
  const base = 'px-1.5 py-0.5 text-xs rounded-full font-medium'
  switch (props.status) {
    case 'extracted':
      return `${base} bg-green-100 text-green-700`
    case 'collected':
      return `${base} bg-blue-100 text-blue-700`
    case 'pending':
      return `${base} bg-gray-100 text-gray-600`
    case 'failed':
      return `${base} bg-red-100 text-red-700`
    default:
      return `${base} bg-gray-100 text-gray-600`
  }
})

const label = computed(() => {
  switch (props.status) {
    case 'extracted':
      return '✓'
    case 'collected':
      return '◐'
    case 'pending':
      return '○'
    case 'failed':
      return '✗'
    default:
      return '?'
  }
})
</script>
