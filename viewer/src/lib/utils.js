/**
 * Truncate text to a maximum length
 */
export function truncate(text, length = 100) {
  if (!text) return ''
  if (text.length <= length) return text
  return text.slice(0, length) + '...'
}

/**
 * Format a timestamp as relative time
 */
export function timeAgo(timestamp) {
  const seconds = Math.floor((Date.now() - timestamp * 1000) / 1000)

  const intervals = [
    { label: 'year', seconds: 31536000 },
    { label: 'month', seconds: 2592000 },
    { label: 'day', seconds: 86400 },
    { label: 'hour', seconds: 3600 },
    { label: 'minute', seconds: 60 },
    { label: 'second', seconds: 1 }
  ]

  for (const interval of intervals) {
    const count = Math.floor(seconds / interval.seconds)
    if (count >= 1) {
      return `${count} ${interval.label}${count !== 1 ? 's' : ''} ago`
    }
  }

  return 'just now'
}

/**
 * Format confidence as percentage
 */
export function formatConfidence(confidence) {
  if (confidence === null || confidence === undefined) return '?'
  return Math.round(confidence * 100) + '%'
}

/**
 * Get confidence color class
 */
export function confidenceColor(confidence) {
  if (confidence >= 0.8) return 'text-green-600'
  if (confidence >= 0.5) return 'text-amber-600'
  return 'text-red-600'
}
