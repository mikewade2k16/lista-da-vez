export interface StorageSettings {
  uploadsEnabled: boolean
  billingCycleDay: number
  storageLimitBytes: number
  classALimit: number
  classBLimit: number
  maxObjectBytes: number
  imageMaxBytes: number
  videoMaxBytes: number
  audioMaxBytes: number
  documentMaxBytes: number
  updatedBy?: string
  updatedAt: string
}

export interface StorageUsage {
  billingMonth: string
  storedBytes: number
  pendingBytes: number
  availableObjects: number
  pendingObjects: number
  classARequests: number
  classBRequests: number
  uploadedBytes: number
}

export interface StorageStatus {
  enabled: boolean
  initialized: boolean
  provider: string
  bucket?: string
  usage: StorageUsage
  cloudUsage: {
    available: boolean
    configured: boolean
    source: string
    windowStart?: string
    windowEnd?: string
    fetchedAt?: string
    storedBytes: number
    metadataBytes: number
    objectCount: number
    classARequests: number
    classBRequests: number
    error?: string
  }
  settings: StorageSettings
}

export interface StorageSettingsInput {
  uploadsEnabled: boolean
  billingCycleDay: number
  storageLimitBytes: number
  classALimit: number
  classBLimit: number
  imageMaxBytes: number
  videoMaxBytes: number
  audioMaxBytes: number
  documentMaxBytes: number
}

export interface StorageObject {
  id: string
  accountId: string
  sourceModule: string
  fileName: string
  contentType: string
  sizeBytes: number
  status: string
  createdAt: string
  availableAt?: string
}
