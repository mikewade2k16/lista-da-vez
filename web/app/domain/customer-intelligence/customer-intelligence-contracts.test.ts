import { describe, expect, it } from 'vitest'
import { getAllowedWorkspaces } from '~/domain/utils/permissions'
import { classifyCustomerApiError } from './api-error'
import { CANONICAL_PROCESS_KEYS } from './prompt-types'

describe('customer intelligence frontend contracts', () => {
  it('keeps exactly the 13 canonical prompt processes', () => {
    expect(CANONICAL_PROCESS_KEYS).toEqual([
      'conversation.triage',
      'conversation.reply',
      'conversation.handoff_summary',
      'memory.extract',
      'profile.summary',
      'recommendation.follow_up',
      'recommendation.offer',
      'recommendation.important_dates',
      'source.suggest',
      'portfolio.opportunity',
      'media.image_analysis',
      'media.document_analysis',
      'quality.review',
    ])
    expect(CANONICAL_PROCESS_KEYS).toHaveLength(13)
    expect(CANONICAL_PROCESS_KEYS).not.toContain('transcription')
    expect(CANONICAL_PROCESS_KEYS).not.toContain('video_summary')
  })

  it('opens the composed workspace with either module permission family', () => {
    expect(getAllowedWorkspaces('custom_role', ['customer_data.subjects.view'], true)).toContain(
      'customer_intelligence',
    )
    expect(
      getAllowedWorkspaces('custom_role', ['customer_intelligence.profile.view'], true),
    ).toContain('customer_intelligence')
    expect(getAllowedWorkspaces('custom_role', ['calendar.view'], true)).not.toContain(
      'customer_intelligence',
    )
  })

  it('classifies capability, authorization and hidden-scope failures safely', () => {
    expect(
      classifyCustomerApiError({
        statusCode: 403,
        data: { error: { reasonCode: 'permission_required' } },
      }).kind,
    ).toBe('forbidden')
    expect(
      classifyCustomerApiError({
        statusCode: 404,
        data: { error: { reasonCode: 'relationship_not_found' } },
      }).kind,
    ).toBe('not_found')
    expect(
      classifyCustomerApiError({
        statusCode: 403,
        data: { error: { reasonCode: 'customer_intelligence_module_disabled' } },
      }).kind,
    ).toBe('capability_off')
  })
})
