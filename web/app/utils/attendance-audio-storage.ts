import {
  buildAttendanceAudioChunkId,
  sortAttendanceAudioChunks,
  type AttendanceAudioChunk,
  type AttendanceAudioSession,
  type AttendanceAudioSessionStatus,
} from '~/domain/operation/attendance-audio'

const DATABASE_NAME = 'omni-attendance-audio-v1'
const DATABASE_VERSION = 1
const SESSION_STORE = 'sessions'
const CHUNK_STORE = 'chunks'
const SESSION_INDEX = 'sessionId'

let databasePromise: Promise<IDBDatabase> | null = null

function requestResult<T>(request: IDBRequest<T>) {
  return new Promise<T>((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error || new Error('Falha no armazenamento local.'))
  })
}

function transactionComplete(transaction: IDBTransaction) {
  return new Promise<void>((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () =>
      reject(transaction.error || new Error('Falha no armazenamento local.'))
    transaction.onabort = () =>
      reject(transaction.error || new Error('Gravacao local interrompida.'))
  })
}

function openDatabase() {
  if (databasePromise) {
    return databasePromise
  }

  if (typeof indexedDB === 'undefined') {
    return Promise.reject(new Error('Este navegador nao oferece armazenamento local para o audio.'))
  }

  databasePromise = new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open(DATABASE_NAME, DATABASE_VERSION)

    request.onupgradeneeded = () => {
      const database = request.result
      if (!database.objectStoreNames.contains(SESSION_STORE)) {
        database.createObjectStore(SESSION_STORE, { keyPath: 'id' })
      }
      if (!database.objectStoreNames.contains(CHUNK_STORE)) {
        const chunks = database.createObjectStore(CHUNK_STORE, { keyPath: 'id' })
        chunks.createIndex(SESSION_INDEX, SESSION_INDEX, { unique: false })
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => {
      databasePromise = null
      reject(request.error || new Error('Nao foi possivel abrir o armazenamento local.'))
    }
    request.onblocked = () => {
      databasePromise = null
      reject(new Error('O armazenamento local esta bloqueado por outra aba.'))
    }
  })

  return databasePromise
}

export function attendanceAudioStorageSupported() {
  return typeof indexedDB !== 'undefined'
}

export async function saveAttendanceAudioSession(session: AttendanceAudioSession) {
  const database = await openDatabase()
  const transaction = database.transaction(SESSION_STORE, 'readwrite')
  transaction.objectStore(SESSION_STORE).put(session)
  await transactionComplete(transaction)
}

export async function appendAttendanceAudioChunk(
  session: AttendanceAudioSession,
  sequence: number,
  blob: Blob,
) {
  const database = await openDatabase()
  const transaction = database.transaction([SESSION_STORE, CHUNK_STORE], 'readwrite')
  const chunk: AttendanceAudioChunk = {
    id: buildAttendanceAudioChunkId(session.id, sequence),
    sessionId: session.id,
    sequence,
    blob,
    createdAt: Date.now(),
  }
  const updatedSession: AttendanceAudioSession = {
    ...session,
    chunkCount: Math.max(session.chunkCount, sequence + 1),
    sizeBytes: session.sizeBytes + blob.size,
  }

  transaction.objectStore(CHUNK_STORE).put(chunk)
  transaction.objectStore(SESSION_STORE).put(updatedSession)
  await transactionComplete(transaction)
  return updatedSession
}

export async function listAttendanceAudioSessions() {
  const database = await openDatabase()
  const transaction = database.transaction(SESSION_STORE, 'readonly')
  const completion = transactionComplete(transaction)
  const sessions = await requestResult(
    transaction.objectStore(SESSION_STORE).getAll() as IDBRequest<AttendanceAudioSession[]>,
  )
  await completion
  return sessions.sort((left, right) => right.startedAt - left.startedAt)
}

export async function getAttendanceAudioChunks(sessionId: string) {
  const database = await openDatabase()
  const transaction = database.transaction(CHUNK_STORE, 'readonly')
  const completion = transactionComplete(transaction)
  const index = transaction.objectStore(CHUNK_STORE).index(SESSION_INDEX)
  const chunks = await requestResult(index.getAll(sessionId) as IDBRequest<AttendanceAudioChunk[]>)
  await completion
  return sortAttendanceAudioChunks(chunks)
}

export async function markAttendanceAudioSession(
  session: AttendanceAudioSession,
  status: AttendanceAudioSessionStatus,
  stopReason: string,
  endedAt = Date.now(),
) {
  const updatedSession: AttendanceAudioSession = {
    ...session,
    endedAt,
    status,
    stopReason,
  }
  await saveAttendanceAudioSession(updatedSession)
  return updatedSession
}

export async function recoverInterruptedAttendanceAudioSessions() {
  const sessions = await listAttendanceAudioSessions()
  const recovered: AttendanceAudioSession[] = []

  for (const session of sessions) {
    if (session.status !== 'recording') {
      recovered.push(session)
      continue
    }

    recovered.push(
      await markAttendanceAudioSession(session, 'interrupted', 'browser-interrupted', Date.now()),
    )
  }

  return recovered.sort((left, right) => right.startedAt - left.startedAt)
}

export async function buildAttendanceAudioBlob(session: AttendanceAudioSession) {
  const chunks = await getAttendanceAudioChunks(session.id)
  if (chunks.length === 0) {
    return null
  }
  return new Blob(
    chunks.map((chunk) => chunk.blob),
    { type: session.mimeType || chunks[0]?.blob.type || 'audio/webm' },
  )
}
