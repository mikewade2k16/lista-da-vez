// Fonte UNICA da taxonomia de conteudo — TIPO + STATUS + PRIORIDADE — compartilhada entre o
// Calendario e o Tasks (WAVE 6). Decisao do dono: os dois modulos oferecem os MESMOS tipos e
// status. O calendario deriva seus *_META daqui (`~/utils/calendar`); o tasks usa a mesma lista no
// seletor de tipo e como colunas padrao de board novo. Arquivo standalone (sem dep de calendar/
// tasks) para nao acoplar os bundles.

export type StatusTone = 'info' | 'success' | 'warning' | 'danger' | 'neutral'

export interface ContentTypeDef {
  value: string
  label: string
  /** Icone i-lucide-* (usado pelo calendario; o tasks usa so label/value). */
  icon: string
}

export interface ContentStatusDef {
  value: string
  label: string
  /** Cor semantica do calendario (pill/badge). */
  tone: StatusTone
  /** Cor da coluna no board do tasks (paleta SUPPORTED_COLUMN_COLORS). */
  color: string
}

export interface ContentPriorityDef {
  value: string
  label: string
  tone: StatusTone
}

// Tipos de conteudo (a ORDEM define a exibicao nos seletores).
export const CONTENT_TYPES: ContentTypeDef[] = [
  { value: 'post', label: 'Post', icon: 'i-lucide-image' },
  { value: 'story', label: 'Story', icon: 'i-lucide-circle-play' },
  { value: 'reels', label: 'Reels', icon: 'i-lucide-film' },
  { value: 'reuniao', label: 'Reuniao', icon: 'i-lucide-users' },
  { value: 'gravacao', label: 'Gravacao', icon: 'i-lucide-video' },
  { value: 'evento', label: 'Evento', icon: 'i-lucide-calendar' },
]

// Status do fluxo de producao (a ORDEM = ordem das colunas do board / dos status no calendario).
export const CONTENT_STATUSES: ContentStatusDef[] = [
  { value: 'planejado', label: 'Planejado', tone: 'neutral', color: 'slate' },
  { value: 'producao', label: 'Producao', tone: 'info', color: 'blue' },
  { value: 'revisao', label: 'Em revisao', tone: 'warning', color: 'amber' },
  { value: 'aprovada', label: 'Aprovada', tone: 'success', color: 'emerald' },
  { value: 'standby', label: 'Standby', tone: 'warning', color: 'violet' },
  { value: 'publicado', label: 'Publicado', tone: 'success', color: 'indigo' },
]

export const CONTENT_PRIORITIES: ContentPriorityDef[] = [
  { value: 'alta', label: 'Alta', tone: 'danger' },
  { value: 'media', label: 'Media', tone: 'warning' },
  { value: 'baixa', label: 'Baixa', tone: 'success' },
]

/** Valores validos (para validacao/guardas). */
export const CONTENT_TYPE_VALUES = CONTENT_TYPES.map((t) => t.value)
export const CONTENT_STATUS_VALUES = CONTENT_STATUSES.map((s) => s.value)
