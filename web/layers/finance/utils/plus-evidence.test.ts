import { expect, it } from 'vitest'
import { evidenceCards, evidenceReviewLabel, plusUsage } from './plus-evidence'
const card = {
  chunkId: 'server-id',
  version: 1,
  excerpt: '<script>untrusted text</script>',
  sourceRef: 'https://example.test/source',
}
it('uses only server evidence fields and preserves excerpt as plain text', () => {
  expect(evidenceCards([card])[0]).toMatchObject({
    href: card.sourceRef,
    excerpt: card.excerpt,
    version: 1,
  })
})
it('does not create executable links for script, data, http or credential URLs', () => {
  for (const sourceRef of [
    'javascript:alert(1)',
    'data:text/html,evil',
    'http://example.test',
    'https://user:secret@example.test',
  ])
    expect(evidenceCards([{ ...card, sourceRef }])[0]?.href).toBe('')
})
it('manual and missing references stay unlinked', () => {
  expect(evidenceCards([{ ...card, sourceRef: 'manual://synthetic/source' }])[0]?.href).toBe('')
  expect(evidenceCards([null, 'fake citation', { chunkId: 'invented' }])).toEqual([])
})
it('rejects invalid document version', () => {
  expect(
    evidenceCards([
      { ...card, version: 0 },
      { ...card, version: '1' },
    ]),
  ).toEqual([])
})
it('unknown cost or usage is never presented as zero', () => {
  expect(plusUsage({ costUsd: null, tokens: null })).toMatchObject({
    cost: 'Custo desconhecido',
    tokens: 'Uso desconhecido',
  })
  expect(plusUsage({ costUsd: '-1', tokens: -1 })?.cost).toBe('Custo desconhecido')
})
it('explicit zero remains distinguishable from unknown and is labeled estimate', () => {
  expect(
    plusUsage({ model: 'server-model', costUsd: '0.000000', tokens: 0, latencyMs: 0 }),
  ).toMatchObject({ cost: 'Custo estimado USD 0.000000', tokens: '0 tokens', latency: '0 ms' })
})

it('labels public source review state and ignores malformed data', () => {
  expect(
    evidenceReviewLabel({ owner: 'approved', aiApproved: 2, aiConcerns: 0, accountant: 'pending' }),
  ).toBe('Revisão: dono aprovado · 2 IA(s) aprovaram · contador pendente')
  expect(evidenceReviewLabel({ owner: 'x', accountant: 'pending' })).toBe('')
  expect(evidenceReviewLabel(undefined)).toBe('')
})
