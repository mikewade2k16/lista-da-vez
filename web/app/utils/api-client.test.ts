import { describe, expect, it, vi } from 'vitest'

import { createApiRequest } from './api-client'

function getFetchMock() {
  return (globalThis as any).$fetch as ReturnType<typeof vi.fn>
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolver) => {
    resolve = resolver
  })

  return { promise, resolve }
}

describe('api-client GET dedupe', () => {
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
})
