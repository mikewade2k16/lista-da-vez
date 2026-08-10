import type { StorageSettings, StorageSettingsInput } from '~/types/storage'

export const STORAGE_LIMITS = Object.freeze({
  storageGigabytes: { min: 0.1, max: 10, step: 0.1 },
  classARequests: { min: 1_000, max: 1_000_000, step: 1_000 },
  classBRequests: { min: 10_000, max: 10_000_000, step: 10_000 },
  fileMebibytes: { min: 1, max: 512, step: 1 },
  billingCycleDay: { min: 1, max: 28, step: 1 },
})

const DECIMAL_GIGABYTE = 1_000_000_000
const MEBIBYTE = 1024 * 1024

export interface StorageLimitsDraft {
  uploadsEnabled: boolean
  billingCycleDay: number
  storageGigabytes: number
  classARequests: number
  classBRequests: number
  imageMebibytes: number
  videoMebibytes: number
  audioMebibytes: number
  documentMebibytes: number
}

export function storageSettingsToDraft(settings: StorageSettings): StorageLimitsDraft {
  return {
    uploadsEnabled: settings.uploadsEnabled,
    billingCycleDay: settings.billingCycleDay,
    storageGigabytes: settings.storageLimitBytes / DECIMAL_GIGABYTE,
    classARequests: settings.classALimit,
    classBRequests: settings.classBLimit,
    imageMebibytes: settings.imageMaxBytes / MEBIBYTE,
    videoMebibytes: settings.videoMaxBytes / MEBIBYTE,
    audioMebibytes: settings.audioMaxBytes / MEBIBYTE,
    documentMebibytes: settings.documentMaxBytes / MEBIBYTE,
  }
}

export function storageDraftToInput(draft: StorageLimitsDraft): StorageSettingsInput {
  return {
    uploadsEnabled: draft.uploadsEnabled,
    billingCycleDay: Math.round(draft.billingCycleDay),
    storageLimitBytes: Math.round(draft.storageGigabytes * DECIMAL_GIGABYTE),
    classALimit: Math.round(draft.classARequests),
    classBLimit: Math.round(draft.classBRequests),
    imageMaxBytes: Math.round(draft.imageMebibytes * MEBIBYTE),
    videoMaxBytes: Math.round(draft.videoMebibytes * MEBIBYTE),
    audioMaxBytes: Math.round(draft.audioMebibytes * MEBIBYTE),
    documentMaxBytes: Math.round(draft.documentMebibytes * MEBIBYTE),
  }
}

export function validateStorageLimitsDraft(draft: StorageLimitsDraft): string {
  for (const value of Object.values(draft).filter((value) => typeof value === 'number')) {
    if (!Number.isFinite(value)) return 'Preencha todos os limites com numeros validos.'
  }

  if (
    draft.billingCycleDay < STORAGE_LIMITS.billingCycleDay.min ||
    draft.billingCycleDay > STORAGE_LIMITS.billingCycleDay.max
  ) {
    return 'O dia inicial do ciclo deve ficar entre 1 e 28.'
  }

  if (
    draft.storageGigabytes < STORAGE_LIMITS.storageGigabytes.min ||
    draft.storageGigabytes > STORAGE_LIMITS.storageGigabytes.max
  ) {
    return 'O armazenamento deve ficar entre 0,1 e 10 GB.'
  }
  if (
    draft.classARequests < STORAGE_LIMITS.classARequests.min ||
    draft.classARequests > STORAGE_LIMITS.classARequests.max
  ) {
    return 'As operacoes Classe A devem ficar entre 1.000 e 1.000.000.'
  }
  if (
    draft.classBRequests < STORAGE_LIMITS.classBRequests.min ||
    draft.classBRequests > STORAGE_LIMITS.classBRequests.max
  ) {
    return 'As operacoes Classe B devem ficar entre 10.000 e 10.000.000.'
  }
  const fileLimits = [
    draft.imageMebibytes,
    draft.videoMebibytes,
    draft.audioMebibytes,
    draft.documentMebibytes,
  ]
  if (
    fileLimits.some(
      (value) =>
        value < STORAGE_LIMITS.fileMebibytes.min || value > STORAGE_LIMITS.fileMebibytes.max,
    )
  ) {
    return 'Cada limite por tipo deve ficar entre 1 e 512 MiB.'
  }

  const input = storageDraftToInput(draft)
  if (
    Math.max(
      input.imageMaxBytes,
      input.videoMaxBytes,
      input.audioMaxBytes,
      input.documentMaxBytes,
    ) > input.storageLimitBytes
  ) {
    return 'Nenhum limite por tipo pode superar o armazenamento total.'
  }
  return ''
}
