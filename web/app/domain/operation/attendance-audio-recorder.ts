import type { AttendanceAudioSession } from './attendance-audio'

export type AttendanceRecorderStatus = 'idle' | 'requesting' | 'recording' | 'stopping' | 'error'

export type AttendanceUploadStatus = 'idle' | 'syncing' | 'saved' | 'error'

export interface StartAttendanceRecordingResult {
  ok: boolean
  code?: string
  message?: string
}

export interface AttendanceRecordingRuntime {
  recorder: MediaRecorder
  session: AttendanceAudioSession
  nextChunkSequence: number
  localPersistenceFailed: boolean
  writeQueue: Promise<void>
  stopPromise: Promise<boolean> | null
}

export interface AttendanceServerRecordingResponse {
  recording?: { id?: string }
}

export interface ActiveAttendanceRecordingsResponse {
  items?: Array<{
    serviceId?: unknown
    recordingStatus?: unknown
  }>
}

export function attendanceMicrophoneErrorMessage(error: unknown) {
  const errorName =
    error instanceof DOMException ? error.name : String((error as { name?: unknown })?.name || '')

  if (errorName === 'NotAllowedError' || errorName === 'PermissionDeniedError') {
    return 'O atendimento iniciou, mas o navegador nao recebeu permissao para usar o microfone.'
  }
  if (errorName === 'NotFoundError' || errorName === 'DevicesNotFoundError') {
    return 'O atendimento iniciou, mas nenhum microfone foi encontrado neste computador.'
  }
  if (errorName === 'NotReadableError' || errorName === 'TrackStartError') {
    return 'O atendimento iniciou, mas o microfone esta ocupado ou indisponivel.'
  }
  return 'O atendimento iniciou, mas nao foi possivel comecar a gravacao do microfone.'
}
