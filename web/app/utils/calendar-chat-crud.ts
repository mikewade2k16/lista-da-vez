// Execucao das propostas de CRUD de ANOTACAO e PERFIL do cliente pelo chat (WAVE 7). Separado de
// useCalendarChat.ts (orquestracao do chat) para respeitar o limite de linhas/arquivo e isolar a
// responsabilidade. A IA NAO grava: estas funcoes rodam no confirm, pela API autenticada do usuario
// (endpoints canonicos notes/{month} e client-profile), reusando o padrao do modal de plano (mes
// ativo => store.setNotesForActiveMonth, que atualiza o painel + persiste; senao GET+aplica+PUT).
import {
  fetchClientProfile,
  fetchNotesForMonth,
  putClientProfile,
  putNotesForMonth,
  type ApiRequest,
} from '~/domain/calendar/calendar-api'
import { defaultClientProfile, type CalendarClientProfile } from '~/utils/calendar'
import type {
  CalendarChatProposalFields,
  CalendarChatProposalNote,
  CalendarChatProposalProfile,
} from '~/domain/calendar/calendar-chat-api'

// Campos do perfil enderecaveis pela IA (WAVE 7): 9 estaveis + 7 do extra.
const PROFILE_STABLE_KEYS = [
  'segment',
  'positioning',
  'description',
  'history',
  'siteUrl',
  'instagram',
  'address',
  'objectives',
  'brandVoice',
] as const
const PROFILE_EXTRA_KEYS = [
  'audience',
  'offer',
  'pillars',
  'cadence',
  'restrictions',
  'performance',
  'assets',
] as const

// Deps injetadas pelo composable: o back e sempre a autoridade; aqui so orquestramos as chamadas.
// store carrega so os 3 pontos de nota usados (evita acoplar ao tipo inteiro da store).
export interface ChatCrudDeps {
  apiRequest: ApiRequest
  store: {
    focusMonthKey: string
    activeNotes: string
    setNotesForActiveMonth: (html: string) => void
  }
  actionableError: (error: unknown) => string
}

// wrapNoteChunk transforma texto simples num paragrafo HTML (a nota do mes e HTML do OmniEditor);
// conteudo que ja vem como HTML (comeca com '<') e mantido como esta.
function wrapNoteChunk(text: string): string {
  const t = String(text || '').trim()
  if (!t) return ''
  if (t.startsWith('<')) return t
  const escaped = t.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return `<p>${escaped.replace(/\n/g, '<br>')}</p>`
}

// appendNote concatena o novo bloco ao conteudo atual (acrescentar; ambos ja sao blocos HTML).
function appendNote(current: string, chunk: string): string {
  const c = String(current || '').trim()
  return c ? `${c}${chunk}` : chunk
}

// mergeProfileFields aplica so os campos NAO-VAZIOS da proposta sobre o perfil atual (merge; o resto
// e preservado — o PUT do back e full-replace, entao mesclamos do lado do front).
function mergeProfileFields(
  target: CalendarClientProfile,
  prof: CalendarChatProposalProfile,
): void {
  const stable = target as unknown as Record<string, string>
  const src = prof as unknown as Record<string, unknown>
  for (const key of PROFILE_STABLE_KEYS) {
    const value = String(src[key] ?? '').trim()
    if (value) stable[key] = value
  }
  if (prof.extra) {
    const extra = target.extra as unknown as Record<string, string>
    const srcExtra = prof.extra as unknown as Record<string, unknown>
    for (const key of PROFILE_EXTRA_KEYS) {
      const value = String(srcExtra[key] ?? '').trim()
      if (value) extra[key] = value
    }
  }
}

// clearProfileField esvazia um campo (estavel ou do extra) do perfil, se a chave for conhecida.
function clearProfileField(target: CalendarClientProfile, key: string): void {
  if ((PROFILE_STABLE_KEYS as readonly string[]).includes(key)) {
    const stable = target as unknown as Record<string, string>
    stable[key] = ''
  } else if ((PROFILE_EXTRA_KEYS as readonly string[]).includes(key)) {
    const extra = target.extra as unknown as Record<string, string>
    extra[key] = ''
  }
}

// applyNoteProposal aplica a proposta de anotacao do mes. Mes ativo => setNotesForActiveMonth
// (atualiza o painel + persiste, fonte unica); senao GET+aplica+PUT. Acrescenta por padrao (append);
// replace reescreve; delete limpa a nota do mes.
export async function applyNoteProposal(
  deps: ChatCrudDeps,
  action: string,
  note: CalendarChatProposalNote,
): Promise<string> {
  const { apiRequest, store, actionableError } = deps
  const month = String(note.month || '').trim() || store.focusMonthKey
  if (!/^\d{4}-\d{2}$/.test(month)) return 'Não consegui identificar o mês da anotação.'
  const chunk = wrapNoteChunk(String(note.content || ''))
  if (action !== 'delete' && !chunk) return 'A anotação proposta veio vazia.'
  try {
    if (month === store.focusMonthKey) {
      const current = String(store.activeNotes || '')
      const next =
        action === 'delete' ? '' : note.mode === 'replace' ? chunk : appendNote(current, chunk)
      store.setNotesForActiveMonth(next)
    } else {
      let next = ''
      if (action !== 'delete') {
        const current = await fetchNotesForMonth(apiRequest, month)
        next = note.mode === 'replace' ? chunk : appendNote(current, chunk)
      }
      await putNotesForMonth(apiRequest, month, next)
    }
    return ''
  } catch (error) {
    return actionableError(error)
  }
}

// applyClientProfileProposal aplica a proposta de perfil: GET->merge->PUT (full-replace), nunca zera
// os campos que a IA nao mexeu. delete: clearAll zera o perfil, clearFields esvazia os listados. O
// clientId vem resolvido pelo cartao (ou fields.clientId como fallback).
export async function applyClientProfileProposal(
  deps: ChatCrudDeps,
  action: string,
  fields: CalendarChatProposalFields,
  clientId: string,
): Promise<string> {
  const { apiRequest, actionableError } = deps
  const targetClientId = String(clientId || fields.clientId || '').trim()
  if (!targetClientId) return 'Escolha o cliente do perfil no cartão antes de aplicar.'
  const prof = fields.profile || {}
  try {
    if (action === 'delete' && prof.clearAll) {
      await putClientProfile(apiRequest, defaultClientProfile(targetClientId))
      return ''
    }
    const current = await fetchClientProfile(apiRequest, targetClientId)
    const next: CalendarClientProfile = {
      ...current,
      clientId: targetClientId,
      extra: { ...current.extra },
    }
    if (action === 'delete') {
      for (const key of prof.clearFields || []) clearProfileField(next, key)
    } else {
      mergeProfileFields(next, prof)
    }
    await putClientProfile(apiRequest, next)
    return ''
  } catch (error) {
    return actionableError(error)
  }
}
