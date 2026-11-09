export interface FinanceSaveState {
  pending: boolean
  saving: boolean
  failed: boolean
}

// Cada trabalho captura seu payload e contexto. A fila nunca consulta a seleção
// atual para decidir onde gravar um trabalho já agendado.
export function createFinanceSaveQueue(delay: number, changed: (state: FinanceSaveState) => void) {
  const jobs = new Map<string, () => Promise<void>>()
  let timer: ReturnType<typeof setTimeout> | undefined
  let running: Promise<boolean> | undefined
  let saving = false
  let failed = false
  const notify = () => changed({ pending: jobs.size > 0, saving, failed })

  async function drain() {
    saving = true
    failed = false
    notify()
    try {
      while (jobs.size) {
        const [key, job] = jobs.entries().next().value!
        jobs.delete(key)
        try {
          await job()
        } catch {
          if (!jobs.has(key)) jobs.set(key, job)
          failed = true
          return false
        }
      }
      return true
    } finally {
      saving = false
      notify()
    }
  }

  async function flush(): Promise<boolean> {
    clearTimeout(timer)
    if (running) return running
    running = drain()
    try {
      return await running
    } finally {
      running = undefined
    }
  }

  function schedule(key: string, job: () => Promise<void>, debounce = delay) {
    jobs.set(key, job)
    failed = false
    notify()
    clearTimeout(timer)
    timer = setTimeout(() => void flush(), debounce)
  }

  return { schedule, flush, has: (key: string) => jobs.has(key) }
}
