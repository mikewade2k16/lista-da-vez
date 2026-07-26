export interface AttendanceTranscription {
  id: string
  storeId: string
  storeName: string
  serviceId: string
  consultantId: string
  consultantName: string
  recordingStatus: 'recording' | 'ready' | 'interrupted' | 'failed' | string
  transcriptionStatus: 'pending' | 'processing' | 'completed' | 'failed' | string
  mimeType: string
  startedAt: number
  endedAt?: number
  chunkCount: number
  sizeBytes: number
  hasAudio: boolean
  transcriptText: string
  transcriptLive: boolean
  liveTranscriptUpdatedAt?: string
  transcriptError?: string
  transcriptionRequested: boolean
  analysisStatus: 'not_requested' | 'pending' | 'processing' | 'completed' | 'failed' | string
  summaryText: string
  analysisReport: AttendanceAnalysisReport
  analysisError?: string
  analysisRequested: boolean
  finishOutcome?: string
  createdAt: string
  updatedAt: string
}

export interface AttendanceAnalysisReport {
  customerIntent?: string
  needs?: string[]
  products?: string[]
  objections?: string[]
  commitments?: string[]
  nextSteps?: string[]
  opportunities?: string[]
  alerts?: string[]
  sentiment?: string
  confidence?: number
}

export interface AttendanceAnalysisConfig {
  enabled: boolean
  transcriptionProvider: 'local'
  transcriptionModel: string
  transcriptionLanguage: string
  credentialId: string
  provider: 'gemini' | 'openai'
  model: string
  systemPrompt: string
  temperature: number
}

export interface AttendanceTranscriptionsResponse {
  items: AttendanceTranscription[]
  total: number
  limit: number
  offset: number
}

export interface AttendanceTranscriptionStoreGroup {
  id: string
  name: string
  items: AttendanceTranscription[]
  consultants: Array<{
    id: string
    name: string
    items: AttendanceTranscription[]
  }>
}

export function groupAttendanceTranscriptions(
  items: AttendanceTranscription[],
): AttendanceTranscriptionStoreGroup[] {
  const stores = new Map<
    string,
    {
      id: string
      name: string
      consultants: Map<string, { id: string; name: string; items: AttendanceTranscription[] }>
    }
  >()
  items.forEach((item) => {
    const store = stores.get(item.storeId) || {
      id: item.storeId,
      name: item.storeName,
      consultants: new Map(),
    }
    const consultant = store.consultants.get(item.consultantId) || {
      id: item.consultantId,
      name: item.consultantName,
      items: [],
    }
    consultant.items.push(item)
    store.consultants.set(item.consultantId, consultant)
    stores.set(item.storeId, store)
  })
  return Array.from(stores.values()).map((store) => ({
    ...store,
    items: Array.from(store.consultants.values())
      .flatMap((consultant) => consultant.items)
      .sort((left, right) => right.startedAt - left.startedAt),
    consultants: Array.from(store.consultants.values()),
  }))
}

export function formatAttendanceTranscriptionDate(timestamp: number) {
  if (!Number.isFinite(timestamp) || timestamp <= 0) return '-'
  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(new Date(timestamp))
}

export function formatAttendanceAudioSize(sizeBytes: number) {
  const size = Math.max(0, Number(sizeBytes || 0))
  if (size < 1024 * 1024) return `${Math.round(size / 1024)} KB`
  return `${(size / (1024 * 1024)).toFixed(1)} MB`
}

export function attendanceTranscriptionStatusLabel(item: AttendanceTranscription) {
  if (item.transcriptionStatus === 'completed') return 'Transcricao concluida'
  if (item.transcriptionStatus === 'processing') return 'Transcrevendo'
  if (item.transcriptionStatus === 'failed') return 'Falha na transcricao'
  if (item.recordingStatus === 'ready' && item.transcriptionRequested) {
    return 'Aguardando Whisper'
  }
  if (item.recordingStatus === 'ready') return 'Audio pronto para transcrever'
  if (item.recordingStatus === 'recording') return 'Recebendo audio'
  if (item.recordingStatus === 'interrupted') return 'Audio interrompido'
  return 'Falha no audio'
}
