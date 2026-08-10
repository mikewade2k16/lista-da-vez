import { describe, expect, it } from 'vitest'

import { storageDraftToInput, storageSettingsToDraft, validateStorageLimitsDraft } from './limits'

describe('storage limits', () => {
  it('converte GB decimal e MiB binario sem perder os valores', () => {
    const input = storageDraftToInput({
      uploadsEnabled: true,
      billingCycleDay: 27,
      storageGigabytes: 9,
      classARequests: 900_000,
      classBRequests: 9_000_000,
      imageMebibytes: 10,
      videoMebibytes: 25,
      audioMebibytes: 15,
      documentMebibytes: 20,
    })

    expect(input).toEqual({
      uploadsEnabled: true,
      billingCycleDay: 27,
      storageLimitBytes: 9_000_000_000,
      classALimit: 900_000,
      classBLimit: 9_000_000,
      imageMaxBytes: 10 * 1024 * 1024,
      videoMaxBytes: 25 * 1024 * 1024,
      audioMaxBytes: 15 * 1024 * 1024,
      documentMaxBytes: 20 * 1024 * 1024,
    })
    expect(storageSettingsToDraft({ ...input, updatedAt: '2026-07-28T00:00:00Z' })).toEqual({
      uploadsEnabled: true,
      billingCycleDay: 27,
      storageGigabytes: 9,
      classARequests: 900_000,
      classBRequests: 9_000_000,
      imageMebibytes: 10,
      videoMebibytes: 25,
      audioMebibytes: 15,
      documentMebibytes: 20,
    })
  })

  it('recusa valores acima da franquia ou do teto seguro por arquivo', () => {
    expect(
      validateStorageLimitsDraft({
        uploadsEnabled: false,
        billingCycleDay: 27,
        storageGigabytes: 10.1,
        classARequests: 900_000,
        classBRequests: 9_000_000,
        imageMebibytes: 25,
        videoMebibytes: 25,
        audioMebibytes: 25,
        documentMebibytes: 25,
      }),
    ).toContain('10 GB')
    expect(
      validateStorageLimitsDraft({
        uploadsEnabled: true,
        billingCycleDay: 27,
        storageGigabytes: 9,
        classARequests: 900_000,
        classBRequests: 9_000_000,
        imageMebibytes: 25,
        videoMebibytes: 513,
        audioMebibytes: 25,
        documentMebibytes: 25,
      }),
    ).toContain('512 MiB')
  })
})
