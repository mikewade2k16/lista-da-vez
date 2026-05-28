interface BffFetchOptions {
  method?: string
  body?: BodyInit | FormData | Record<string, unknown> | null
  query?: Record<string, unknown>
  headers?: HeadersInit
  signal?: AbortSignal
}

export function useBffFetch() {
  const sessionSimulation = useSessionSimulationStore()

  async function bffFetch<T>(request: string, options: BffFetchOptions = {}) {
    const headers = new Headers(options.headers)

    for (const [key, value] of Object.entries(sessionSimulation.requestHeaders)) {
      if (!value) {
        continue
      }

      headers.set(key, value)
    }

    return await $fetch<T>(request, {
      method: options.method,
      body: options.body as never,
      query: options.query,
      headers,
      signal: options.signal,
    })
  }

  return {
    bffFetch,
  }
}
