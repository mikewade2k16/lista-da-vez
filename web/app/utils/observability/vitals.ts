import { createClsTracker } from './cls'
import type { ObsEvent } from './events'

// Coleta de Web Vitals por PerformanceObserver (sem libs). So roda no browser.
// `emit` recebe os eventos prontos; o plugin decide fila/envio.

type Emit = (event: ObsEvent) => void

interface PerfEntry extends PerformanceEntry {
  value?: number
  hadRecentInput?: boolean
  interactionId?: number
  processingStart?: number
  size?: number
}

function observe(type: string, handler: (entries: PerfEntry[]) => void, init = {}): boolean {
  try {
    const observer = new PerformanceObserver((list) => handler(list.getEntries() as PerfEntry[]))
    observer.observe({ type, buffered: true, ...init } as PerformanceObserverInit)
    return true
  } catch {
    return false
  }
}

export function createVitalsCollector(emit: Emit) {
  const cls = createClsTracker()
  let lcp = 0
  let lcpDone = false
  let inp = 0

  function vital(name: string, value: number) {
    if (!Number.isFinite(value) || value < 0) return
    emit({ kind: 'vital', name, value: Math.round(value * 1000) / 1000 })
  }

  function finalizeLcp() {
    if (lcpDone) return
    lcpDone = true
    if (lcp > 0) vital('LCP', lcp)
  }

  function start() {
    observe('largest-contentful-paint', (entries) => {
      if (lcpDone) return
      const last = entries[entries.length - 1]
      if (last) lcp = last.startTime
    })
    observe('layout-shift', (entries) => {
      for (const entry of entries) {
        cls.add({
          value: entry.value || 0,
          startTime: entry.startTime,
          hadRecentInput: entry.hadRecentInput,
        })
      }
    })
    observe(
      'event',
      (entries) => {
        for (const entry of entries) {
          if ((entry.interactionId || 0) > 0 && entry.duration > inp) inp = entry.duration
        }
      },
      { durationThreshold: 40 },
    )

    // LCP deixa de valer no primeiro input do usuario.
    for (const type of ['keydown', 'pointerdown']) {
      window.addEventListener(type, finalizeLcp, { once: true, capture: true })
    }

    const reportNavigation = () => {
      const nav = performance.getEntriesByType('navigation')[0] as
        | PerformanceNavigationTiming
        | undefined
      if (!nav) return
      vital('TTFB', nav.responseStart)
      if (nav.loadEventEnd > 0) {
        emit({
          kind: 'pageload',
          name: 'load',
          value: Math.round(nav.loadEventEnd),
          detail: { spa: false },
        })
      }
    }
    if (document.readyState === 'complete') {
      setTimeout(reportNavigation, 0)
    } else {
      window.addEventListener('load', () => setTimeout(reportNavigation, 0), { once: true })
    }
  }

  // Emite o que ficou pendente (CLS, INP, LCP) e zera para a proxima pagina.
  function flush() {
    finalizeLcp()
    if (cls.value() > 0) vital('CLS', cls.value())
    if (inp > 0) vital('INP', inp)
    cls.reset()
    inp = 0
  }

  return { start, flush }
}

// Tempo de troca de rota (SPA) ate o proximo frame ocioso.
export function measureSpaPageload(startedAt: number, emit: Emit) {
  const done = () => {
    emit({
      kind: 'pageload',
      name: 'spa',
      value: Math.round(performance.now() - startedAt),
      detail: { spa: true },
    })
  }
  requestAnimationFrame(() => {
    if (typeof requestIdleCallback === 'function') {
      requestIdleCallback(done, { timeout: 3000 })
    } else {
      setTimeout(done, 0)
    }
  })
}
