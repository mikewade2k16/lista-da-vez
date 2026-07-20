import { ref } from 'vue'

type BootstrapHandle = {
  id: number
  startedAt: number
}

export function usePageBootstrapLoading(
  options: {
    minimumDelayMs?: number
    initiallyLoading?: boolean
  } = {},
) {
  const minimumDelayMs = Math.max(0, Math.trunc(options.minimumDelayMs ?? 450))
  const pageLoading = ref(options.initiallyLoading ?? true)
  let activeBootstrapId = 0

  function wait(delayMs: number) {
    return new Promise<void>((resolve) => {
      setTimeout(resolve, delayMs)
    })
  }

  function startPageBootstrap(): BootstrapHandle {
    const handle = {
      id: ++activeBootstrapId,
      startedAt: Date.now(),
    }

    pageLoading.value = true
    return handle
  }

  async function finishPageBootstrap(handle: BootstrapHandle | null | undefined) {
    if (!handle) {
      return
    }

    const elapsedMs = Date.now() - handle.startedAt
    const remainingMs = minimumDelayMs - elapsedMs

    if (remainingMs > 0) {
      await wait(remainingMs)
    }

    if (handle.id === activeBootstrapId) {
      pageLoading.value = false
    }
  }

  async function runPageBootstrap<T>(task: () => Promise<T>) {
    const handle = startPageBootstrap()

    try {
      return await task()
    } finally {
      await finishPageBootstrap(handle)
    }
  }

  return {
    pageLoading,
    startPageBootstrap,
    finishPageBootstrap,
    runPageBootstrap,
  }
}
