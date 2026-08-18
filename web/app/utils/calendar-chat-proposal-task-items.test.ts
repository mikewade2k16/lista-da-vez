import { describe, expect, it } from 'vitest'
import type { CalendarChatStoredProposal } from '~/domain/calendar/calendar-chat-api'
import { editableFields } from './calendar-chat-proposal-edit'
import {
  calendarProposalChanges,
  calendarProposalTargetClientId,
  calendarProposalTargetTitle,
} from './calendar-chat-proposal-preview'

const proposal: CalendarChatStoredProposal = {
  id: '0',
  action: 'update',
  kind: 'taskItem',
  status: 'pending',
  fields: {
    targetId: 'task-1',
    taskItem: {
      id: 'item-1',
      itemTitle: 'Reel antigo',
      taskTitle: 'Conteúdos Cliente X',
      title: 'Reel principal',
      status: 'posted',
      statusDate: '2026-08-13',
      completed: false,
    },
  },
}

const context = {
  clients: [],
  calendarItems: [],
  people: [],
  getEventById: () => null,
}

describe('preview de proposta taskItem', () => {
  it('usa os snapshots autoritativos da task e não exige cliente', () => {
    expect(calendarProposalTargetTitle(proposal, context)).toBe('Conteúdos Cliente X')
    expect(calendarProposalTargetClientId(proposal, context)).toBe('')
  })

  it('mostra status, data, finalização e novo título com rótulos humanos', () => {
    expect(calendarProposalChanges(proposal, context)).toEqual([
      {
        key: 'taskItem.title',
        label: 'Item',
        before: 'Reel antigo',
        after: 'Reel principal',
      },
      { key: 'taskItem.status', label: 'Status', before: '', after: 'Postado' },
      { key: 'taskItem.statusDate', label: 'Data do status', before: '', after: '13 de ago.' },
      { key: 'taskItem.completed', label: 'Finalizado', before: '', after: 'Não' },
    ])
  })

  it('oferece editor tipado apenas para os campos propostos', () => {
    expect(editableFields(proposal)).toEqual([
      { key: 'taskItem.title', label: 'Item', kind: 'text', path: 'taskItem.title' },
      {
        key: 'taskItem.status',
        label: 'Status',
        kind: 'taskStatus',
        path: 'taskItem.status',
      },
      {
        key: 'taskItem.statusDate',
        label: 'Data do status',
        kind: 'date',
        path: 'taskItem.statusDate',
      },
      {
        key: 'taskItem.completed',
        label: 'Finalizado',
        kind: 'boolean',
        path: 'taskItem.completed',
      },
    ])
  })
})
