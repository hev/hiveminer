<template>
  <div v-if="!store.currentThread" class="p-8 text-center text-gray-400">
    <div v-if="store.currentThreadState">
      <p>Thread not collected yet</p>
      <p class="text-sm mt-2">Status: {{ store.currentThreadState.status }}</p>
    </div>
    <div v-else>
      Select a thread to view
    </div>
  </div>

  <div v-else class="p-6">
    <!-- Post Header -->
    <div class="mb-6">
      <h1 class="text-xl font-semibold text-gray-900 mb-2">
        {{ store.currentThread.post.title }}
      </h1>
      <div class="flex items-center gap-3 text-sm text-gray-500">
        <span class="text-orange-600">r/{{ store.currentThread.post.subreddit }}</span>
        <span>{{ store.currentThread.post.score }} points</span>
        <span>{{ store.currentThread.post.num_comments }} comments</span>
        <span>u/{{ store.currentThread.post.author }}</span>
      </div>
    </div>

    <!-- Post Content -->
    <div v-if="store.currentThread.post.selftext" class="bg-white rounded-lg p-4 mb-6 border">
      <div class="prose prose-sm max-w-none text-gray-700 whitespace-pre-wrap">
        {{ store.currentThread.post.selftext }}
      </div>
    </div>

    <!-- Comments -->
    <div class="space-y-4">
      <h2 class="text-sm font-semibold text-gray-500 uppercase">
        Comments ({{ flatComments.length }})
      </h2>

      <div v-for="comment in flatComments" :key="comment.id" class="bg-white rounded-lg p-4 border">
        <div class="flex items-center gap-2 mb-2 text-sm">
          <span class="font-medium text-gray-900">u/{{ comment.author }}</span>
          <span class="text-gray-400">·</span>
          <span class="text-gray-500">{{ comment.score }} points</span>
          <span v-if="comment.depth > 0" class="text-gray-400 text-xs">
            (reply depth: {{ comment.depth }})
          </span>
        </div>
        <div class="text-gray-700 text-sm whitespace-pre-wrap">
          {{ comment.body }}
        </div>
      </div>

      <div v-if="flatComments.length === 0" class="text-gray-400 text-sm text-center py-4">
        No comments
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useThreadStore } from '../stores/threads'

const store = useThreadStore()

// Flatten comments for display
const flatComments = computed(() => {
  if (!store.currentThread?.comments) return []
  return flattenComments(store.currentThread.comments)
})

function flattenComments(comments, result = []) {
  for (const comment of comments) {
    result.push(comment)
    if (comment.replies?.length) {
      flattenComments(comment.replies, result)
    }
  }
  return result
}
</script>
