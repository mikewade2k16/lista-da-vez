import { afterEach, describe, expect, it, vi } from 'vitest'

import { createApiRequest, setApiLoadingHooks } from './api-client'

function getFetchMock() {
  return (globalThis as any).$fetch as ReturnType<typeof vi.fn>
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolver, rejecter) => {
    resolve = resolver
    reject = rejecter
  })

  return { promise, resolve, reject }
}

describe('api-client GET dedupe', () => {
  afterEach(() => {
    setApiLoadingHooks(null)
    vi.useRealTimers()
  })

  it('does not share in-flight requests when query values differ', async () => {
    const fetchMock = getFetchMock()
    fetchMock.mockImplementation((_path, options) =>
      Promise.resolve({ board: String(options?.query?.board || '') }),
    )

    const api = createApiRequest({
      apiInternalBase: 'http://api.internal',
      public: { apiBase: 'http://api.public' },
    })

    const first = api('/v1/tasks', { query: { board: 'a' } })
    const second = api('/v1/tasks', { query: { board: 'b' } })

    expect(fetchMock).toHaveBeenCalledTimes(2)
    await expect(first).resolves.toEqual({ board: 'a' })
    await expect(second).resolves.toEqual({ board: 'b' })
  })

  it('shares in-flight requests when path and query are equivalent', async () => {
    const fetchMock = getFetchMock()
    const deferred = createDeferred({ board: 'shared' })
    fetchMock.mockReturnValueOnce(deferred.promise)

    const api = createApiRequest({
      apiInternalBase: 'http://api.internal',
      public: { apiBase: 'http://api.public' },
    })

    const first = api('/v1/tasks', { query: { b: '2', a: '1' } })
    const second = api('/v1/tasks?b=2', { query: { a: '1' } })

    expect(fetchMock).toHaveBeenCalledTimes(1)

    deferred.resolve({ board: 'shared' })

    await expect(Promise.all([first, second])).resolves.toEqual([
      { board: 'shared' },
      { board: 'shared' },
    ])
  })

  it('settles loading hooks without creating an unhandled rejection on failed requests', async () => {
    vi.useFakeTimers()

    const fetchMock = getFetchMock()
    const deferred = createDeferred({ ok: true })
    const requestError = Object.assign(new Error('Forbidden'), { statusCode: 403 })
    fetchMock.mockReturnValueOnce(deferred.promise)

    const push = vi.fn()
    const pop = vi.fn()
    setApiLoadingHooks({ push, pop })

    const api = createApiRequest({
      apiInternalBase: 'http://api.internal',
      public: { apiBase: 'http://api.public' },
    })

    const request = api('/v1/alerts/rules')
    await vi.advanceTimersByTimeAsync(201)

    expect(push).toHaveBeenCalledTimes(1)

    deferred.reject(requestError)

    await expect(request).rejects.toBe(requestError)
    await Promise.resolve()

    expect(pop).toHaveBeenCalledTimes(1)
  })
})
