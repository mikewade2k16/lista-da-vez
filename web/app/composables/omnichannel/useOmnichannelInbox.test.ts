import { describe, expect, it, vi } from 'vitest'
import { createOmnichannelActiveAccountChangeHandler } from './useOmnichannelInbox'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

describe('active account scope barrier', () => {
  it('clears synchronously, bootstraps REST and rejects an old in-flight response', async () => {
    let generation = 0
    let projection = ['account-a']
    const oldGeneration = generation
    const oldResponse = deferred<string>()
    const bootstrapResponse = deferred<true>()
    const oldRequest = oldResponse.promise.then((value) => {
      if (generation === oldGeneration) {
        projection = [value]
      }
    })
    const bootstrapRest = vi.fn(async () => {
      await bootstrapResponse.promise
      projection = ['account-b']
    })
    const handler = createOmnichannelActiveAccountChangeHandler({
      advanceScopeGeneration: () => {
        generation += 1
      },
      clearScope: () => {
        projection = []
      },
      bootstrapRest,
    })

    const accountChange = handler('account-b', 'account-a')

    expect(generation).toBe(1)
    expect(projection).toEqual([])
    expect(bootstrapRest).toHaveBeenCalledTimes(1)

    oldResponse.resolve('stale-account-a')
    await oldRequest
    expect(projection).toEqual([])

    bootstrapResponse.resolve(true)
    await expect(accountChange).resolves.toBe(true)
    expect(projection).toEqual(['account-b'])
  })

  it('advances the identity epoch for A to B to A and ignores identical ids', async () => {
    let generation = 0
    const handler = createOmnichannelActiveAccountChangeHandler({
      advanceScopeGeneration: () => {
        generation += 1
      },
      clearScope: vi.fn(),
      bootstrapRest: vi.fn().mockResolvedValue(undefined),
    })

    await expect(handler('account-a', 'account-a')).resolves.toBe(false)
    await expect(handler('account-b', 'account-a')).resolves.toBe(true)
    await expect(handler('account-a', 'account-b')).resolves.toBe(true)
    expect(generation).toBe(2)
  })
})
