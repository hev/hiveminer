<template>
  <div class="p-3">
    <h2 class="text-sm font-semibold text-gray-500 uppercase mb-3">Threads</h2>

    <div class="space-y-2">
      <div
        v-for="(thread, index) in store.threads"
        :key="thread.post_id"
        @click="store.selectThread(index)"
        :class="[
          'p-3 rounded cursor-pointer border transition-colors',
          index === store.currentThreadIndex
            ? 'bg-orange-50 border-orange-300'
            : 'bg-white border-gray-200 hover:border-gray-300'
        ]"
      >
        <div class="flex items-start gap-2">
          <StatusBadge :status="thread.status" />
          <div class="flex-1 min-w-0">
            <div class="text-sm font-medium text-gray-900 truncate">
              {{ thread.title }}
            </div>
            <div class="text-xs text-gray-500 mt-1">
              r/{{ thread.subreddit }} · {{ thread.score }} pts · {{ thread.num_comments }} comments
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="store.threads.length === 0" class="text-gray-400 text-sm text-center py-8">
      No threads found
    </div>
  </div>
</template>

<script setup>
import { useThreadStore } from '../stores/threads'
import StatusBadge from './StatusBadge.vue'

const store = useThreadStore()
</script>
