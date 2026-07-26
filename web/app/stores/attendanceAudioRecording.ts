import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import {
  chooseAttendanceAudioMimeType,
  createAttendanceAudioSession,
  type AttendanceAudioServiceReference,
  type AttendanceAudioSession,
  type AttendanceAudioSessionStatus,
} from '~/domain/operation/attendance-audio'
import {
  attendanceMicrophoneErrorMessage,
  type ActiveAttendanceRecordingsResponse,
  type AttendanceRecordingRuntime,
  type AttendanceRecorderStatus,
  type AttendanceServerRecordingResponse,
  type AttendanceUploadStatus,
  type StartAttendanceRecordingResult,
} from '~/domain/operation/attendance-audio-recorder'
import { useAuthStore } from '~/stores/auth'
import {
  appendAttendanceAudioChunk,
  attendanceAudioStorageSupported,
  getAttendanceAudioChunks,
  markAttendanceAudioSession,
  recoverInterruptedAttendanceAudioSessions,
  saveAttendanceAudioSession,
} from '~/utils/attendance-audio-storage'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const CHUNK_INTERVAL_MS = 5_000

export const useAttendanceAudioRecordingStore = defineStore('attendanceAudioRecording', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const initialized = ref(false)
  const status = ref<AttendanceRecorderStatus>('idle')
  const uploadStatus = ref<AttendanceUploadStatus>('idle')
  const errorMessage = ref('')
  const uploadErrorMessage = ref('')
  const sessionsByServiceId = ref<Record<string, AttendanceAudioSession>>({})
  const latestSession = ref<AttendanceAudioSession | null>(null)
  const activeRecordingServiceIds = ref<string[]>([])

  const runtimes = new Map<string, AttendanceRecordingRuntime>()
  let sharedMediaStream: MediaStream | null = null
  let sharedMediaStreamPromise: Promise<MediaStream> | null = null
  let initializePromise: Promise<boolean> | null = null
  let pendingStarts = 0
  let beforeUnloadBound = false

  const localRecordingServiceIds = computed(() => Object.keys(sessionsByServiceId.value))
  const isRecording = computed(() => localRecordingServiceIds.value.length > 0)
  const isBusy = computed(
    () => status.value === 'requesting' || status.value === 'stopping' || pendingStarts > 0,
  )

  function accountHeaders(accountId: string) {
    const normalizedAccountId = String(accountId || '').trim()
    return normalizedAccountId ? { 'X-Account-Id': normalizedAccountId } : {}
  }

  function accountIdForStore(storeId: string) {
    const normalizedStoreId = String(storeId || '').trim()
    const store = (auth.storeContext || []).find(
      (candidate) => String(candidate?.id || '').trim() === normalizedStoreId,
    )
    return String(store?.tenantId || auth.activeTenantId || '').trim()
  }

  function accountIdsForVisibleStores() {
    const ids = (auth.storeContext || [])
      .map((store) => String(store?.tenantId || '').trim())
      .filter(Boolean)
    const fallback = String(auth.activeTenantId || '').trim()
    if (fallback) ids.push(fallback)
    return Array.from(new Set(ids))
  }

  function setRuntimeSession(serviceId: string, session: AttendanceAudioSession) {
    const runtime = runtimes.get(serviceId)
    if (runtime) runtime.session = session
    sessionsByServiceId.value = { ...sessionsByServiceId.value, [serviceId]: session }
  }

  function removeRuntimeSession(serviceId: string) {
    sessionsByServiceId.value = Object.fromEntries(
      Object.entries(sessionsByServiceId.value).filter(([key]) => key !== serviceId),
    )
  }

  function requestFinalChunksBeforeUnload() {
    runtimes.forEach(({ recorder }) => {
      if (recorder.state !== 'recording') return
      try {
        recorder.requestData()
      } catch {
        // Os checkpoints de 5s continuam sendo a protecao principal.
      }
    })
  }

  async function initialize() {
    if (initialized.value) return true
    if (initializePromise) return initializePromise

    initializePromise = (async () => {
      if (import.meta.server) return false
      if (!attendanceAudioStorageSupported()) {
        status.value = 'error'
        errorMessage.value =
          'Este navegador nao oferece o armazenamento local necessario para gravar em partes.'
        return false
      }

      try {
        const sessions = await recoverInterruptedAttendanceAudioSessions()
        latestSession.value =
          sessions.find((session) => session.chunkCount > 0 && session.sizeBytes > 0) ||
          sessions[0] ||
          null
        initialized.value = true
        if (!beforeUnloadBound) {
          window.addEventListener('beforeunload', requestFinalChunksBeforeUnload)
          beforeUnloadBound = true
        }
        return true
      } catch {
        status.value = 'error'
        errorMessage.value = 'Nao foi possivel preparar o armazenamento local da gravacao.'
        return false
      }
    })()

    const result = await initializePromise
    initializePromise = null
    return result
  }

  async function acquireMicrophone() {
    if (sharedMediaStream?.active) return sharedMediaStream
    if (sharedMediaStreamPromise) return sharedMediaStreamPromise

    sharedMediaStreamPromise = navigator.mediaDevices
      .getUserMedia({ audio: true })
      .then((stream) => {
        sharedMediaStream = stream
        return stream
      })
      .finally(() => {
        sharedMediaStreamPromise = null
      })
    return sharedMediaStreamPromise
  }

  function releaseMicrophoneIfIdle() {
    if (runtimes.size > 0 || pendingStarts > 0) return
    sharedMediaStream?.getTracks().forEach((track) => track.stop())
    sharedMediaStream = null
  }

  async function ensureServerRecording(session: AttendanceAudioSession) {
    if (session.serverRecordingId) return session
    const accountId = session.accountId || accountIdForStore(session.storeId)
    const response = (await apiRequest('/v1/operations/transcriptions', {
      method: 'POST',
      headers: accountHeaders(accountId),
      body: {
        storeId: session.storeId,
        serviceId: session.serviceId,
        clientSessionId: session.id,
        mimeType: session.mimeType,
        startedAt: session.startedAt,
      },
      skipLoadingIndicator: true,
    })) as AttendanceServerRecordingResponse
    const serverRecordingId = String(response?.recording?.id || '').trim()
    if (!serverRecordingId) throw new Error('A API nao devolveu o identificador da gravacao.')

    const updated = { ...session, accountId, serverRecordingId }
    await saveAttendanceAudioSession(updated)
    return updated
  }

  async function refreshActiveRecordings() {
    const accountIds = accountIdsForVisibleStores()
    const scopes = accountIds.length ? accountIds : ['']
    const responses = await Promise.allSettled(
      scopes.map(
        async (accountId) =>
          (await apiRequest('/v1/operations/transcriptions?limit=100&offset=0', {
            headers: accountHeaders(accountId),
            dedupe: false,
            skipLoadingIndicator: true,
          })) as ActiveAttendanceRecordingsResponse,
      ),
    )
    const remoteIds = responses.flatMap((result) =>
      result.status === 'fulfilled'
        ? (result.value.items || [])
            .filter((item) => item.recordingStatus === 'recording')
            .map((item) => String(item.serviceId || '').trim())
            .filter(Boolean)
        : [],
    )
    activeRecordingServiceIds.value = Array.from(new Set(remoteIds))
    return responses.some((result) => result.status === 'fulfilled')
  }

  async function uploadChunk(
    runtime: AttendanceRecordingRuntime,
    session: AttendanceAudioSession,
    sequence: number,
    blob: Blob,
  ) {
    const serverSession = await ensureServerRecording(session)
    await apiRequest(
      `/v1/operations/transcriptions/${encodeURIComponent(serverSession.serverRecordingId)}/chunks/${sequence}`,
      {
        method: 'PUT',
        headers: {
          ...accountHeaders(serverSession.accountId),
          'Content-Type': session.mimeType || blob.type || 'audio/webm',
        },
        body: blob,
        skipLoadingIndicator: true,
      },
    )
    const updated = {
      ...serverSession,
      uploadedChunkCount: Math.max(serverSession.uploadedChunkCount || 0, sequence + 1),
    }
    try {
      await saveAttendanceAudioSession(updated)
    } catch {
      runtime.localPersistenceFailed = true
    }
    return updated
  }

  function enqueueChunk(serviceId: string, blob: Blob) {
    const runtime = runtimes.get(serviceId)
    if (!runtime || blob.size === 0) return

    const sequence = runtime.nextChunkSequence
    runtime.nextChunkSequence += 1
    runtime.writeQueue = runtime.writeQueue.then(async () => {
      let session = runtime.session
      try {
        session = await appendAttendanceAudioChunk(session, sequence, blob)
        setRuntimeSession(serviceId, session)
      } catch {
        runtime.localPersistenceFailed = true
        errorMessage.value =
          'Uma parte nao pode ser salva no navegador; o envio seguro ao servidor continua.'
      }

      uploadStatus.value = 'syncing'
      try {
        session = await uploadChunk(runtime, session, sequence, blob)
        setRuntimeSession(serviceId, session)
        uploadStatus.value = 'idle'
        uploadErrorMessage.value = ''
      } catch (error) {
        uploadStatus.value = 'error'
        uploadErrorMessage.value = getApiErrorMessage(
          error,
          'Uma parte ainda nao chegou ao servidor. Tentaremos novamente ao parar.',
        )
      }
    })
  }

  async function startForService(
    service: AttendanceAudioServiceReference,
  ): Promise<StartAttendanceRecordingResult> {
    const serviceId = String(service.serviceId || '').trim()
    if (!serviceId) return { ok: false, code: 'invalid-service', message: 'Atendimento invalido.' }
    if (runtimes.has(serviceId)) return { ok: true, code: 'already-recording-service' }

    errorMessage.value = ''
    uploadErrorMessage.value = ''
    status.value = 'requesting'
    pendingStarts += 1

    let session: AttendanceAudioSession | null = null
    try {
      if (!(await initialize())) {
        return { ok: false, code: 'storage-unavailable', message: errorMessage.value }
      }
      if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === 'undefined') {
        throw new DOMException('MediaRecorder unavailable', 'NotSupportedError')
      }

      const stream = await acquireMicrophone()
      const mimeType = chooseAttendanceAudioMimeType((candidate) =>
        MediaRecorder.isTypeSupported(candidate),
      )
      const recorder = mimeType
        ? new MediaRecorder(stream, { mimeType })
        : new MediaRecorder(stream)
      session = createAttendanceAudioSession(
        {
          ...service,
          accountId:
            String(service.accountId || '').trim() ||
            accountIdForStore(String(service.storeId || '')),
        },
        recorder.mimeType || mimeType,
      )
      await saveAttendanceAudioSession(session)
      session = await ensureServerRecording(session)

      const runtime: AttendanceRecordingRuntime = {
        recorder,
        session,
        nextChunkSequence: 0,
        localPersistenceFailed: false,
        writeQueue: Promise.resolve(),
        stopPromise: null,
      }
      runtimes.set(serviceId, runtime)
      setRuntimeSession(serviceId, session)
      recorder.addEventListener('dataavailable', (event) => enqueueChunk(serviceId, event.data))
      recorder.addEventListener('error', () => {
        errorMessage.value =
          'O navegador interrompeu um gravador. As partes ja salvas continuam disponiveis.'
        void stopForService(serviceId, 'recorder-error', 'failed')
      })
      recorder.start(CHUNK_INTERVAL_MS)
      status.value = 'recording'
      uploadStatus.value = 'idle'
      latestSession.value = null
      return { ok: true }
    } catch (error) {
      status.value = runtimes.size ? 'recording' : 'error'
      errorMessage.value =
        getApiErrorMessage(error, '') ||
        (error instanceof DOMException
          ? attendanceMicrophoneErrorMessage(error)
          : 'O servidor nao aceitou a gravacao deste atendimento.')
      if (session) {
        try {
          latestSession.value = await markAttendanceAudioSession(session, 'failed', 'start-failed')
        } catch {
          latestSession.value = { ...session, status: 'failed', stopReason: 'start-failed' }
        }
      }
      return { ok: false, code: 'recording-start-failed', message: errorMessage.value }
    } finally {
      pendingStarts -= 1
      releaseMicrophoneIfIdle()
    }
  }

  async function syncSessionToServer(
    runtime: AttendanceRecordingRuntime,
    session: AttendanceAudioSession,
    endedAt: number,
  ) {
    uploadStatus.value = 'syncing'
    let serverSession = await ensureServerRecording(session)
    const chunks = await getAttendanceAudioChunks(session.id)
    for (const chunk of chunks) {
      serverSession = await uploadChunk(runtime, serverSession, chunk.sequence, chunk.blob)
    }
    await apiRequest(
      `/v1/operations/transcriptions/${encodeURIComponent(serverSession.serverRecordingId)}/complete`,
      {
        method: 'POST',
        headers: accountHeaders(serverSession.accountId),
        body: { endedAt },
        skipLoadingIndicator: true,
      },
    )
    uploadStatus.value = 'saved'
    return serverSession
  }

  async function stopForService(
    serviceId: string,
    reason = 'manual',
    finalStatus: AttendanceAudioSessionStatus = 'complete',
  ) {
    const runtime = runtimes.get(String(serviceId || '').trim())
    if (!runtime) return false
    if (runtime.stopPromise) return runtime.stopPromise

    runtime.stopPromise = (async () => {
      status.value = 'stopping'
      if (runtime.recorder.state !== 'inactive') {
        await new Promise<void>((resolve) => {
          runtime.recorder.addEventListener('stop', () => resolve(), { once: true })
          try {
            runtime.recorder.stop()
          } catch {
            resolve()
          }
        })
      }
      await runtime.writeQueue

      runtimes.delete(serviceId)
      removeRuntimeSession(serviceId)
      releaseMicrophoneIfIdle()

      const endedAt = Date.now()
      const resolvedStatus: AttendanceAudioSessionStatus = runtime.localPersistenceFailed
        ? 'failed'
        : finalStatus
      let completedSession: AttendanceAudioSession
      try {
        completedSession = await markAttendanceAudioSession(
          runtime.session,
          resolvedStatus,
          reason,
          endedAt,
        )
      } catch {
        completedSession = {
          ...runtime.session,
          endedAt,
          status: resolvedStatus,
          stopReason: reason,
        }
      }
      latestSession.value = completedSession

      try {
        latestSession.value = await syncSessionToServer(runtime, completedSession, endedAt)
        uploadErrorMessage.value = ''
      } catch (error) {
        uploadStatus.value = 'error'
        uploadErrorMessage.value = getApiErrorMessage(
          error,
          'O audio ficou local e ainda nao foi consolidado no servidor.',
        )
      }
      status.value = runtimes.size ? 'recording' : 'idle'
      return uploadStatus.value === 'saved'
    })()
    return runtime.stopPromise
  }

  async function stopAll(
    reason = 'manual',
    finalStatus: AttendanceAudioSessionStatus = 'complete',
  ) {
    const results = await Promise.all(
      Array.from(runtimes.keys()).map((serviceId) =>
        stopForService(serviceId, reason, finalStatus),
      ),
    )
    return results.every(Boolean)
  }

  async function reconcileActiveServices(
    services: Array<{ serviceId?: unknown; stoppedAt?: unknown }>,
  ) {
    const activeById = new Map(
      services.map((service) => [String(service.serviceId || '').trim(), service]),
    )
    const stops: Promise<boolean>[] = []
    runtimes.forEach((_runtime, serviceId) => {
      const service = activeById.get(serviceId)
      if (!service) {
        stops.push(stopForService(serviceId, 'service-ended'))
      } else if (Number(service.stoppedAt || 0) > 0) {
        stops.push(stopForService(serviceId, 'service-stopped'))
      }
    })
    await Promise.all(stops)
  }

  return {
    initialized,
    status,
    uploadStatus,
    errorMessage,
    uploadErrorMessage,
    sessionsByServiceId,
    localRecordingServiceIds,
    latestSession,
    activeRecordingServiceIds,
    isRecording,
    isBusy,
    chunkIntervalMs: CHUNK_INTERVAL_MS,
    initialize,
    refreshActiveRecordings,
    startForService,
    stopForService,
    stopAll,
    reconcileActiveServices,
  }
})
