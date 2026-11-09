import { expect, it, vi } from 'vitest'
import {
  createKnowledgeController,
  type KnowledgeContext,
  type KnowledgeTransport,
} from './knowledge-controller'
import { emptyKnowledgeMetadata, type KnowledgeDocument } from '../../types/knowledge'
function setup() {
  const context: KnowledgeContext = {
    accountId: 'account-a',
    actorId: 'actor-a',
    issuerId: 'issuer-a',
    modules: ['accounting'],
    permissions: ['accounting.knowledge.view', 'accounting.knowledge.manage'],
  }
  const api = vi.fn().mockResolvedValue([])
  return {
    context,
    api,
    controller: createKnowledgeController(() => context, api as KnowledgeTransport),
  }
}
const document = (): KnowledgeDocument => ({
  id: 'doc-a',
  baseId: 'base-a',
  title: 'Servidor',
  sourceRef: 'manual://fixture/test',
  checksum: 'hash',
  status: 'draft',
  version: 2,
  lockVersion: 4,
  metadata: emptyKnowledgeMetadata(),
  reviewedBy: null,
  chunkCount: 1,
})
it('gates module, scope and effective knowledge permission before fetching', async () => {
  const { controller, context, api } = setup()
  context.permissions = []
  await controller.load()
  expect(api).not.toHaveBeenCalled()
  context.permissions = ['accounting.knowledge.view']
  await controller.createBase('forbidden')
  expect(api).not.toHaveBeenCalled()
})
it('loads authoritative base and document versions', async () => {
  const { controller, api } = setup()
  api.mockResolvedValueOnce([{ id: 'base-a', name: 'Servidor', enabled: false, version: 7 }])
  await controller.load()
  expect(controller.state.bases[0]?.version).toBe(7)
  api.mockResolvedValueOnce([document()])
  await controller.choose('base-a')
  expect(controller.state.documents[0]?.lockVersion).toBe(4)
})
it('discard late documents after account/issuer reset', async () => {
  const { controller, context, api } = setup()
  let resolve!: (value: unknown) => void
  api.mockImplementationOnce(
    () =>
      new Promise((r) => {
        resolve = r
      }),
  )
  const pending = controller.choose('base-a')
  context.accountId = 'account-b'
  context.issuerId = 'issuer-b'
  controller.reset()
  resolve([document()])
  expect(await pending).toBe(false)
  expect(controller.state.documents).toEqual([])
})
it('review persists metadata and uses CAS without publishing', async () => {
  const { controller, api } = setup()
  const doc = document()
  controller.state.documents = [doc]
  api.mockResolvedValue({ ...doc, lockVersion: 5, reviewedBy: 'actor-a' })
  await controller.review(doc, { ...doc.metadata, authority: 'Conferida' })
  expect(api.mock.calls[0]?.[2].body).toMatchObject({ expectedVersion: 4, confirmed: true })
  expect(api.mock.calls[0]?.[1]).toContain('/review')
  expect(controller.state.documents[0]?.lockVersion).toBe(5)
  expect(controller.state.documents[0]?.status).toBe('draft')
})
it('conflict preserves loaded version for explicit reload', async () => {
  const { controller, api } = setup()
  const doc = document()
  controller.state.documents = [doc]
  api.mockRejectedValue(new Error('409'))
  expect(await controller.publish(doc)).toBe(false)
  expect(controller.state.documents[0]?.lockVersion).toBe(4)
  expect(controller.state.error).toContain('recarregue')
})
it('search has a bounded context without granting confirmed applicability', async () => {
  const { controller, api } = setup()
  api.mockResolvedValue({
    status: 'insufficient_evidence',
    evidence: [],
    message: 'Sem evidência',
    potentialConflict: false,
  })
  await controller.search('serviço', '2026-11-04', 'ZZ', 'unknown')
  expect(api.mock.calls[0]?.[0]).toBe('account-a')
  expect(api.mock.calls[0]?.[1]).toContain('/issuer-a/knowledge/search')
  expect(api.mock.calls[0]?.[2].body).toMatchObject({ topK: 5, regime: 'unknown' })
  expect(api.mock.calls[0]?.[2].body).not.toHaveProperty('contextConfirmed')
  expect(controller.state.result?.status).toBe('insufficient_evidence')
})
