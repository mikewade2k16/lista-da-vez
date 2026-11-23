// CLS por janelas de sessao (metodologia do web.dev): shifts agrupados quando o
// intervalo entre eles e < 1s e a janela dura no maximo 5s. O CLS e a maior janela.

export interface LayoutShiftLike {
  value: number
  startTime: number
  hadRecentInput?: boolean
}

const GAP_MS = 1000
const WINDOW_MS = 5000

export function createClsTracker() {
  let windowValue = 0
  let windowStart = 0
  let lastShiftTime = 0
  let best = 0

  return {
    add(entry: LayoutShiftLike) {
      if (entry.hadRecentInput) return
      const startsNewWindow =
        windowValue === 0 ||
        entry.startTime - lastShiftTime > GAP_MS ||
        entry.startTime - windowStart > WINDOW_MS
      if (startsNewWindow) {
        windowValue = entry.value
        windowStart = entry.startTime
      } else {
        windowValue += entry.value
      }
      lastShiftTime = entry.startTime
      if (windowValue > best) best = windowValue
    },
    value() {
      return best
    },
    reset() {
      windowValue = 0
      windowStart = 0
      lastShiftTime = 0
      best = 0
    },
  }
}
