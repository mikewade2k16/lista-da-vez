import { describe, expect, it } from 'vitest'

import {
  attendanceTranscriptionStatusLabel,
  formatAttendanceAudioSize,
  groupAttendanceTranscriptions,
  type AttendanceTranscription,
} from './attendance-transcriptions'

function recording(overrides: Partial<AttendanceTranscription> = {}): AttendanceTranscription {
  return {
    id: 'recording-1',
    storeId: 'store-1',
    storeName: 'Loja Centro',
    serviceId: 'service-1',
    consultantId: 'consultant-1',
    consultantName: 'Ana',
    recordingStatus: 'ready',
    transcriptionStatus: 'pending',
    mimeType: 'audio/webm',
    startedAt: 1,
    chunkCount: 2,
    sizeBytes: 2048,
    hasAudio: true,
    transcriptText: '',
    transcriptLive: false,
    transcriptionRequested: false,
    analysisStatus: 'not_requested',
    summaryText: '',
    analysisReport: {},
    analysisRequested: false,
    createdAt: '2026-07-24T12:00:00Z',
    updatedAt: '2026-07-24T12:01:00Z',
    ...overrides,
  }
}

describe('attendance transcriptions', () => {
  it('deixa explicito quando o audio esta pronto mas o Whisper ainda nao rodou', () => {
    expect(attendanceTranscriptionStatusLabel(recording())).toBe('Audio pronto para transcrever')
  })

  it('prioriza o estado da transcricao quando ela existir', () => {
    expect(
      attendanceTranscriptionStatusLabel(recording({ transcriptionStatus: 'completed' })),
    ).toBe('Transcricao concluida')
  })

  it('formata o tamanho sem inventar precisao', () => {
    expect(formatAttendanceAudioSize(2048)).toBe('2 KB')
    expect(formatAttendanceAudioSize(1.5 * 1024 * 1024)).toBe('1.5 MB')
  })

  it('separa os atendimentos por loja e consultor', () => {
    const groups = groupAttendanceTranscriptions([
      recording({ id: '1', storeId: 'store-a', storeName: 'Centro', consultantId: 'ana' }),
      recording({ id: '2', storeId: 'store-a', storeName: 'Centro', consultantId: 'bia' }),
      recording({ id: '3', storeId: 'store-b', storeName: 'Shopping', consultantId: 'ana' }),
    ])

    expect(groups).toHaveLength(2)
    expect(groups[0]?.consultants).toHaveLength(2)
    expect(groups[1]?.consultants[0]?.items[0]?.id).toBe('3')
  })
})
