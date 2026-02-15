import { defineStore } from 'pinia'

export const useThreadStore = defineStore('threads', {
  state: () => ({
    manifest: null,
    currentThreadIndex: 0,
    currentThread: null,
    loading: false,
    error: null,
    threadCache: {}
  }),

  getters: {
    threads: (state) => state.manifest?.threads || [],

    currentThreadState: (state) => {
      if (!state.manifest || state.currentThreadIndex < 0) return null
      return state.manifest.threads[state.currentThreadIndex]
    },

    extractedThreads: (state) => {
      if (!state.manifest) return []
      return state.manifest.threads.filter(t => t.status === 'extracted' || t.status === 'ranked')
    },

    rankedEntries: (state) => {
      if (!state.manifest) return []
      const entries = []
      for (const thread of state.manifest.threads) {
        if ((thread.status === 'extracted' || thread.status === 'ranked') && thread.entries) {
          for (const entry of thread.entries) {
            entries.push({ ...entry, _thread: thread })
          }
        }
      }
      // Sort by rank_score descending, unscored last
      entries.sort((a, b) => {
        if (a.rank_score == null && b.rank_score == null) return 0
        if (a.rank_score == null) return 1
        if (b.rank_score == null) return -1
        return b.rank_score - a.rank_score
      })
      return entries
    }
  },

  actions: {
    async loadManifest() {
      this.loading = true
      this.error = null

      try {
        const res = await fetch('/api/manifest')
        if (!res.ok) throw new Error('Failed to load manifest')
        this.manifest = await res.json()

        // Auto-select first thread
        if (this.manifest.threads.length > 0) {
          await this.selectThread(0)
        }
      } catch (err) {
        this.error = err.message
      } finally {
        this.loading = false
      }
    },

    async selectThread(index) {
      if (!this.manifest || index < 0 || index >= this.manifest.threads.length) return

      this.currentThreadIndex = index
      const threadState = this.manifest.threads[index]

      // Check cache
      if (this.threadCache[threadState.post_id]) {
        this.currentThread = this.threadCache[threadState.post_id]
        return
      }

      // Fetch thread
      try {
        const res = await fetch(`/api/thread/${threadState.post_id}`)
        if (!res.ok) {
          this.currentThread = null
          return
        }
        const thread = await res.json()
        this.threadCache[threadState.post_id] = thread
        this.currentThread = thread
      } catch (err) {
        console.error('Failed to load thread:', err)
        this.currentThread = null
      }
    },

    nextThread() {
      if (!this.manifest) return
      const next = Math.min(this.currentThreadIndex + 1, this.manifest.threads.length - 1)
      this.selectThread(next)
    },

    prevThread() {
      if (!this.manifest) return
      const prev = Math.max(this.currentThreadIndex - 1, 0)
      this.selectThread(prev)
    }
  }
})
