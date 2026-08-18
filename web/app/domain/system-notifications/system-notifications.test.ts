import { describe, expect, it } from 'vitest'
import { buildSystemNotifications } from './system-notifications'

describe('buildSystemNotifications', () => {
  it('agrega as quatro fontes e identifica o módulo de origem', () => {
    const result = buildSystemNotifications({
      inApp: [
        {
          id: 'n1',
          sourceModule: 'tasks',
          title: 'Nova tarefa',
          body: 'Você foi atribuído.',
          linkPath: '/tasks',
          createdAt: '2026-08-18T10:00:00Z',
          payload: { severity: 'warning' },
        },
      ],
      feedback: [
        {
          id: 'f1',
          subject: 'Erro no painel',
          status: 'open',
          unread_count: 2,
          last_message_body: 'Respondemos seu chamado.',
          last_message_at: '2026-08-18T11:00:00Z',
        },
      ],
      contentBrief: {
        generatedAt: '2026-08-18T12:00:00Z',
        today: '2026-08-18',
        mode: 'follow_up',
        headline: 'Acompanhamento',
        summary: 'Resumo',
        counts: { critical: 1, attention: 0, info: 0, total: 1 },
        clients: [],
        alerts: [
          {
            id: 'c1',
            type: 'client_without_post',
            severity: 'critical',
            title: 'Sem postagem recente',
            body: 'Cliente precisa de conteúdo.',
            clientId: 'client-1',
            clientName: 'Cliente A',
            linkPath: '/calendario',
          },
        ],
      },
      operationalAlerts: [
        {
          id: 'a1',
          status: 'active',
          severity: 'info',
          sourceModule: 'operations',
          headline: 'Atendimento longo',
          body: 'Revise o atendimento.',
          createdAt: '2026-08-18T09:00:00Z',
        },
      ],
    })

    expect(result).toHaveLength(4)
    expect(result.map((item) => item.sourceLabel)).toEqual([
      'Operação de conteúdo',
      'Tasks',
      'Chamados',
      'Operação',
    ])
    expect(result[0]).toMatchObject({ bucket: 'content', severity: 'critical' })
  })

  it('descarta notificações lidas, chamados sem não-lidos e alertas encerrados', () => {
    const result = buildSystemNotifications({
      inApp: [{ id: 'read', readAt: '2026-08-18T10:00:00Z' }],
      feedback: [{ id: 'feedback', status: 'closed', unread_count: 3 }],
      operationalAlerts: [{ id: 'resolved', status: 'resolved' }],
    })

    expect(result).toEqual([])
  })

  it('preserva ids por fonte para evitar colisões entre módulos', () => {
    const result = buildSystemNotifications({
      inApp: [{ id: 'same', title: 'Sistema' }],
      feedback: [{ id: 'same', status: 'open', unread_count: 1 }],
      operationalAlerts: [{ id: 'same', status: 'active' }],
    })

    expect(new Set(result.map((item) => item.id)).size).toBe(3)
  })
})
