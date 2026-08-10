import { describe, expect, it } from 'vitest'
import { normalizeTaskVideoItem, taskVideoSignature } from './task-video'

describe('task video metadata', () => {
  it('preserves the checklist item association', () => {
    const video = normalizeTaskVideoItem({
      id: 'video-1',
      name: 'original.mp4',
      url: '/uploads/tasks/account/video-1/original.mp4',
      size: 125,
      contentType: 'video/mp4',
      checklistItemId: 'item-1',
      uploadedAt: '2026-07-29T21:00:00Z',
    })

    expect(video?.checklistItemId).toBe('item-1')
  })

  it('includes the association in the persistence signature', () => {
    const base = {
      id: 'video-1',
      name: 'original.mp4',
      url: '/uploads/tasks/account/video-1/original.mp4',
      size: 125,
      contentType: 'video/mp4',
      uploadedAt: '2026-07-29T21:00:00Z',
    }

    expect(taskVideoSignature([{ ...base, checklistItemId: 'item-1' }])).not.toBe(
      taskVideoSignature([{ ...base, checklistItemId: 'item-2' }]),
    )
  })
})
