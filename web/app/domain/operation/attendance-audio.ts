export type AttendanceAudioSessionStatus = 'recording' | 'complete' | 'interrupted' | 'failed'

export interface AttendanceAudioSession {
  id: string
  accountId: string
  serviceId: string
  storeId: string
  personId: string
  personName: string
  startedAt: number
  endedAt: number | null
  status: AttendanceAudioSessionStatus
  mimeType: string
  chunkCount: number
  sizeBytes: number
  stopReason: string
  serverRecordingId: string
  uploadedChunkCount: number
}

export interface AttendanceAudioChunk {
  id: string
  sessionId: string
  sequence: number
  blob: Blob
  createdAt: number
}

export interface AttendanceAudioServiceReference {
  accountId?: unknown
  serviceId?: unknown
  storeId?: unknown
  id?: unknown
  personId?: unknown
  name?: unknown
  personName?: unknown
}

const MIME_TYPE_CANDIDATES = [
  'audio/webm;codecs=opus',
  'audio/webm',
  'audio/mp4;codecs=mp4a.40.2',
  'audio/mp4',
]

function normalizeText(value: unknown) {
  return String(value || '').trim()
}

export function chooseAttendanceAudioMimeType(isTypeSupported: (mimeType: string) => boolean) {
  return MIME_TYPE_CANDIDATES.find((mimeType) => isTypeSupported(mimeType)) || ''
}

export function createAttendanceAudioSession(
  service: AttendanceAudioServiceReference,
  mimeType: string,
  now = Date.now(),
  sessionId = '',
): AttendanceAudioSession {
  const serviceId = normalizeText(service.serviceId)
  const generatedId =
    sessionId || `${serviceId || 'service'}-${now}-${Math.random().toString(36).slice(2, 10)}`

  return {
    id: generatedId,
    accountId: normalizeText(service.accountId),
    serviceId,
    storeId: normalizeText(service.storeId),
    personId: normalizeText(service.personId || service.id),
    personName: normalizeText(service.personName || service.name) || 'Consultor',
    startedAt: now,
    endedAt: null,
    status: 'recording',
    mimeType: normalizeText(mimeType),
    chunkCount: 0,
    sizeBytes: 0,
    stopReason: '',
    serverRecordingId: '',
    uploadedChunkCount: 0,
  }
}

export function buildAttendanceAudioChunkId(sessionId: string, sequence: number) {
  return `${sessionId}:${String(Math.max(0, sequence)).padStart(8, '0')}`
}

export function sortAttendanceAudioChunks(chunks: AttendanceAudioChunk[]) {
  return [...chunks].sort((left, right) => left.sequence - right.sequence)
}

export function buildAttendanceAudioFilename(session: AttendanceAudioSession) {
  const safeName =
    normalizeText(session.personName)
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
      .replace(/[^a-zA-Z0-9_-]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .toLowerCase() || 'atendimento'
  const date = new Date(session.startedAt).toISOString().replace(/[:.]/g, '-')
  const extension = session.mimeType.includes('mp4') ? 'm4a' : 'webm'
  return `atendimento-${safeName}-${date}.${extension}`
}

export function formatAttendanceRecordingDuration(durationMs: number) {
  const totalSeconds = Math.max(0, Math.floor(Number(durationMs || 0) / 1000))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const parts = [minutes, seconds].map((part) => String(part).padStart(2, '0'))

  if (hours > 0) {
    parts.unshift(String(hours).padStart(2, '0'))
  }

  return parts.join(':')
}
