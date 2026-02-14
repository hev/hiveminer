<template>
  <div class="p-3">
    <h2 class="text-sm font-semibold text-gray-500 uppercase mb-3">Extracted Fields</h2>

    <div v-if="!threadState" class="text-gray-400 text-sm text-center py-8">
      Select a thread
    </div>

    <div v-else-if="threadState.status !== 'extracted'" class="text-gray-400 text-sm text-center py-8">
      <p>Not extracted yet</p>
      <p class="mt-2">Status: {{ threadState.status }}</p>
    </div>

    <div v-else class="space-y-4">
      <FieldValue
        v-for="field in threadState.fields"
        :key="field.id"
        :field="field"
      />

      <div v-if="!threadState.fields?.length" class="text-gray-400 text-sm text-center py-4">
        No fields extracted
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
</script>
