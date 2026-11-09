import { expect, it, vi } from 'vitest'
import {
  createIngestionController,
  type IngestionContext,
  type IngestionTransport,
} from './ingestion-controller'
const context = (): IngestionContext => ({
  accountId: 'a',
  actorId: 'u',
  issuerId: 'i',
  baseId: 'b',
  modules: ['accounting'],
  permissions: ['accounting.knowledge.view', 'accounting.knowledge.manage'],
})
it('does not read or configure without access', async () => {
  const api = vi.fn(),
    c = createIngestionController(() => ({ ...context(), permissions: [] }), api)
  await c.load()
  await c.save()
  expect(api).not.toHaveBeenCalled()
})
it('starts without a selected source or network activation', () => {
  const api = vi.fn(),
    c = createIngestionController(context, api)
  expect(c.state.enabled).toBe(false)
  expect(c.state.sourceId).toBe('')
  expect(api).not.toHaveBeenCalled()
})
it('loads authoritative catalog, subscription and review history', async () => {
  const api = vi.fn().mockImplementation((_account, path) =>
    Promise.resolve(
      path.includes('catalog')
        ? [{ id: 'source', title: 'Official selection' }]
        : path.includes('subscriptions')
          ? [
              {
                id: 's',
                baseId: 'b',
                sourceId: 'source',
                version: 2,
                enabled: true,
                intervalHours: 6,
              },
            ]
          : { items: [{ id: 'r', baseId: 'b', state: 'review_pending' }] },
    ),
  )
  const c = createIngestionController(context, api)
  await c.load()
  c.select('source')
  expect(c.state.enabled).toBe(true)
  expect(c.state.intervalHours).toBe(6)
  expect(c.state.runs[0]?.state).toBe('review_pending')
})
it('keeps the same idempotency key after uncertain configuration write', async () => {
  const api = vi.fn().mockRejectedValue(new Error('timeout')),
    key = vi.fn(() => 'durable-key')
  const c = createIngestionController(context, api, key)
  c.state.sourceId = 'source'
  c.state.enabled = true
  await c.save()
  await c.save()
  expect(api).toHaveBeenCalledTimes(2)
  expect(key).toHaveBeenCalledTimes(1)
  expect(api.mock.calls[1]?.[2]).toMatchObject({
    key: 'durable-key',
    body: { sourceId: 'source', baseId: 'b', enabled: true, expectedVersion: 0 },
  })
})
it('prevents follow-up and old state after account/base change during save', async () => {
  let ctx = context(),
    finish!: (v: unknown) => void
  const api = vi.fn(
    () =>
      new Promise((resolve) => {
        finish = resolve
      }),
  )
  const c = createIngestionController(() => ctx, api as IngestionTransport)
  c.state.sourceId = 'source'
  const save = c.save()
  ctx = { ...ctx, accountId: 'another' }
  c.reset()
  finish({ id: 'old' })
  await save
  expect(api).toHaveBeenCalledTimes(1)
  expect(c.state.catalog).toEqual([])
  expect(c.state.subscriptions).toEqual([])
  expect(c.state.busy).toBe(false)
})
