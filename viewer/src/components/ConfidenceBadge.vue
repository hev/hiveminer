<template>
  <span :class="badgeClasses">
    {{ formatted }}
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  confidence: {
    type: Number,
    required: true
  }
})

const formatted = computed(() => {
  if (props.confidence === null || props.confidence === undefined) return '?'
  return Math.round(props.confidence * 100) + '%'
})

const badgeClasses = computed(() => {
  const base = 'px-1.5 py-0.5 text-xs rounded font-medium'
  const c = props.confidence

  if (c >= 0.8) return `${base} bg-green-100 text-green-700`
  if (c >= 0.5) return `${base} bg-amber-100 text-amber-700`
  return `${base} bg-red-100 text-red-700`
})
</script>
