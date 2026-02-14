<template>
  <div class="h-screen flex flex-col">
    <!-- Header -->
    <header class="bg-orange-600 text-white px-4 py-3 flex items-center justify-between">
      <div class="flex items-center gap-4">
        <h1 class="text-xl font-bold">Threadminer</h1>
        <span v-if="store.manifest" class="text-orange-200">{{ store.manifest.form.title }}</span>
      </div>
      <div class="text-sm text-orange-200">
        {{ extractedCount }}/{{ totalCount }} extracted
      </div>
    </header>

    <!-- Loading State -->
    <div v-if="store.loading" class="flex-1 flex items-center justify-center">
      <div class="text-gray-500">Loading...</div>
    </div>

    <!-- Error State -->
    <div v-else-if="store.error" class="flex-1 flex items-center justify-center">
      <div class="text-red-500">{{ store.error }}</div>
    </div>

    <!-- Main Content -->
    <div v-else-if="store.manifest" class="flex-1 flex overflow-hidden">
      <!-- Left Sidebar - Thread List -->
      <div class="w-80 bg-white border-r overflow-y-auto">
        <ThreadList />
      </div>

      <!-- Center - Thread View -->
      <div class="flex-1 overflow-y-auto bg-gray-50">
        <ThreadView />
      </div>

      <!-- Right Sidebar - Extracted Fields -->
      <div class="w-80 bg-white border-l overflow-y-auto">
        <FieldPanel />
      </div>
    </div>

    <!-- Footer -->
    <footer class="bg-gray-200 px-4 py-2 text-sm text-gray-600 flex gap-4">
      <span><kbd class="px-1 bg-gray-300 rounded">j</kbd>/<kbd class="px-1 bg-gray-300 rounded">k</kbd> navigate threads</span>
      <span><kbd class="px-1 bg-gray-300 rounded">Enter</kbd> view thread</span>
    </footer>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted } from 'vue'
import { useThreadStore } from './stores/threads'
import ThreadList from './components/ThreadList.vue'
import ThreadView from './components/ThreadView.vue'
import FieldPanel from './components/FieldPanel.vue'

const store = useThreadStore()

const extractedCount = computed(() => {
  if (!store.manifest) return 0
  return store.manifest.threads.filter(t => t.status === 'extracted').length
})

const totalCount = computed(() => {
  if (!store.manifest) return 0
  return store.manifest.threads.length
})

// Keyboard navigation
function handleKeydown(e) {
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return

  switch (e.key) {
    case 'j':
    case 'ArrowDown':
      e.preventDefault()
      store.nextThread()
      break
    case 'k':
    case 'ArrowUp':
      e.preventDefault()
      store.prevThread()
      break
  }
}

onMounted(() => {
  store.loadManifest()
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>
