import { describe, expect, it } from 'vitest'

import {
  buildAttendanceAudioFilename,
  chooseAttendanceAudioMimeType,
  createAttendanceAudioSession,
  formatAttendanceRecordingDuration,
  sortAttendanceAudioChunks,
  type AttendanceAudioChunk,
} from './attendance-audio'

describe('attendance audio', () => {
  it('prefere opus e usa fallback suportado pelo navegador', () => {
    expect(chooseAttendanceAudioMimeType((mimeType) => mimeType.includes('opus'))).toBe(
      'audio/webm;codecs=opus',
    )
    expect(chooseAttendanceAudioMimeType((mimeType) => mimeType === 'audio/mp4')).toBe('audio/mp4')
    expect(chooseAttendanceAudioMimeType(() => false)).toBe('')
  })

  it('vincula a sessao local ao atendimento confirmado', () => {
    const session = createAttendanceAudioSession(
      {
        accountId: 'account-1',
        serviceId: 'service-42',
        storeId: 'store-7',
        id: 'person-3',
        name: 'Ana Souza',
      },
      'audio/webm;codecs=opus',
      1_700_000_000_000,
      'session-fixed',
    )

    expect(session).toMatchObject({
      id: 'session-fixed',
      accountId: 'account-1',
      serviceId: 'service-42',
      storeId: 'store-7',
      personId: 'person-3',
      personName: 'Ana Souza',
      status: 'recording',
      chunkCount: 0,
      sizeBytes: 0,
    })
  })

  it('ordena os blocos antes de reconstruir o audio', () => {
    const chunks = [
      { id: '2', sessionId: 's', sequence: 2 },
      { id: '0', sessionId: 's', sequence: 0 },
      { id: '1', sessionId: 's', sequence: 1 },
    ] as AttendanceAudioChunk[]

    expect(sortAttendanceAudioChunks(chunks).map((chunk) => chunk.sequence)).toEqual([0, 1, 2])
  })

  it('gera duracao e nome de download previsiveis', () => {
    const session = createAttendanceAudioSession(
      { serviceId: 'service-9', name: 'Joao da Silva' },
      'audio/mp4',
      Date.UTC(2026, 6, 24, 12, 30, 0),
      'session-fixed',
    )

    expect(formatAttendanceRecordingDuration(65_000)).toBe('01:05')
    expect(formatAttendanceRecordingDuration(3_665_000)).toBe('01:01:05')
    expect(buildAttendanceAudioFilename(session)).toBe(
      'atendimento-joao-da-silva-2026-07-24T12-30-00-000Z.m4a',
    )
  })
})
