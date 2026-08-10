import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const dialogSource = readFileSync(
  new URL('./OperationPauseReasonDialog.vue', import.meta.url),
  'utf8',
)
const pickerSource = readFileSync(new URL('./OperationProductPicker.vue', import.meta.url), 'utf8')
const operationStyles = readFileSync(
  new URL('../../assets/styles/components/operation.css', import.meta.url),
  'utf8',
)

describe('operation pause reason dialog portal layer contract', () => {
  it('renders the teleported picker above the high-priority dialog backdrop', () => {
    expect(operationStyles).toMatch(/\.ui-dialog-backdrop\s*\{[\s\S]*?z-index:\s*11000;/)
    expect(dialogSource).toContain('const PAUSE_REASON_PORTAL_BASE_Z_INDEX = 11001')
    expect(dialogSource).toContain(':portal-base-z-index="PAUSE_REASON_PORTAL_BASE_Z_INDEX"')
    expect(pickerSource).toContain(':style="portalScrimStyle"')
    expect(pickerSource).toContain(':style="portalDropdownStyle"')
    expect(pickerSource).toContain('zIndex: normalizedPortalBaseZIndex.value,')
    expect(pickerSource).toContain('zIndex: normalizedPortalBaseZIndex.value + 1,')
  })
})
