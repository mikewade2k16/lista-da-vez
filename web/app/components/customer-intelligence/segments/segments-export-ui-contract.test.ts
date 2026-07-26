import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const workspaceSource = readFileSync(
  new URL('./CustomerSegmentsWorkspace.vue', import.meta.url),
  'utf8',
)
const materializationsSource = readFileSync(
  new URL('./SegmentMaterializationsPanel.vue', import.meta.url),
  'utf8',
)
const segmentsComposableSource = readFileSync(
  new URL('../../../composables/customer-intelligence/useCustomerSegments.ts', import.meta.url),
  'utf8',
)

describe('customer segments export UI contract', () => {
  it('does not expose the unavailable segment export request', () => {
    expect(workspaceSource).not.toContain('SegmentExportDialog')
    expect(workspaceSource).not.toContain('requestExport')
    expect(workspaceSource).not.toContain('@export')
    expect(segmentsComposableSource).not.toContain('requestExport')
  })

  it('labels current materializations as read-only until the governed API exists', () => {
    expect(materializationsSource).toContain('Exportacao ainda indisponivel')
    expect(materializationsSource).toMatch(/API\s+governada ser implementada/)
    expect(materializationsSource).not.toContain("emit('export'")
  })
})
