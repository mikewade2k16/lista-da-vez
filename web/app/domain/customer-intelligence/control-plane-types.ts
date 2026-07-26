export const INTELLIGENCE_CAPABILITY_DEFINITIONS = [
  {
    key: 'customer_intelligence.profile',
    label: 'Perfil e memoria inteligente',
    description: 'Leitura de perfil, fatos, contexto e sincronizacao de fontes.',
    modes: ['off', 'shadow', 'on'],
  },
  {
    key: 'customer_intelligence.runtime',
    label: 'Runtime de IA',
    description: 'Executa processos publicados em shadow, canary ou producao.',
    modes: ['off', 'shadow', 'canary', 'on'],
  },
  {
    key: 'customer_intelligence.portfolio',
    label: 'Portfolio agregado',
    description: 'Oportunidades protegidas entre clientes, sem contributors.',
    modes: ['off', 'shadow', 'on'],
  },
] as const

export type IntelligenceCapabilityKey = (typeof INTELLIGENCE_CAPABILITY_DEFINITIONS)[number]['key']

export interface IntelligenceCapabilityView {
  id?: string
  accountId: string
  clientAccountId: string
  key: IntelligenceCapabilityKey
  scopeKey: string
  mode: string
  config: Record<string, unknown>
  revision: number
  updatedAt?: string
}

export interface IntelligenceCapabilityWriteInput {
  clientAccountId: string
  scopeKey: string
  mode: string
  config: Record<string, unknown>
  expectedRevision: number
}

export const CUSTOMER_DATA_CAPABILITY_DEFINITIONS = [
  {
    key: 'core',
    label: 'Nucleo de dados',
    description: 'Relacionamentos, notas e consentimentos autoritativos.',
    modes: ['off', 'shadow', 'on'],
  },
  {
    key: 'identity_resolution',
    label: 'Resolucao de identidade',
    description: 'Identidades protegidas e associacao deterministica de pessoas.',
    modes: ['off', 'shadow', 'on'],
  },
  {
    key: 'matching_merge',
    label: 'Matching e merge',
    description: 'Candidatos, decisao assistida e unificacao reversivel.',
    modes: ['off', 'shadow', 'on'],
  },
  {
    key: 'offline_interactions',
    label: 'Interacoes offline',
    description: 'Ligacoes, reunioes e demais contatos fora dos canais online.',
    modes: ['off', 'shadow', 'on'],
  },
  {
    key: 'segmentation',
    label: 'Segmentacao',
    description: 'Definicoes, avaliacoes e materializacoes de segmentos.',
    modes: ['off', 'shadow', 'on'],
  },
  {
    key: 'segment_exports',
    label: 'Exportacao de segmentos',
    description: 'Saida controlada de audiencias para destinos autorizados.',
    modes: ['off', 'shadow', 'on'],
  },
] as const

export type CustomerDataCapabilityKey = (typeof CUSTOMER_DATA_CAPABILITY_DEFINITIONS)[number]['key']
export type CustomerDataCapabilityMode =
  (typeof CUSTOMER_DATA_CAPABILITY_DEFINITIONS)[number]['modes'][number]

export const CUSTOMER_DATA_WRITER_DEFINITIONS = [
  {
    key: 'relationship',
    label: 'Relacionamento',
    capabilityKey: 'core',
  },
  {
    key: 'identity',
    label: 'Identidade',
    capabilityKey: 'identity_resolution',
  },
  {
    key: 'note',
    label: 'Nota',
    capabilityKey: 'core',
  },
  {
    key: 'consent',
    label: 'Consentimento',
    capabilityKey: 'core',
  },
  {
    key: 'merge',
    label: 'Merge',
    capabilityKey: 'matching_merge',
  },
  {
    key: 'segment_definition',
    label: 'Definicao de segmento',
    capabilityKey: 'segmentation',
  },
] as const

export type CustomerDataWriterKey = (typeof CUSTOMER_DATA_WRITER_DEFINITIONS)[number]['key']
export type CustomerDataWriterMode = 'legacy' | 'shadow' | 'new'

export interface CustomerDataCapabilityState {
  capabilityKey: CustomerDataCapabilityKey
  mode: CustomerDataCapabilityMode
  revision: number
  updatedAt?: string
}

export interface CustomerDataWriterState {
  entityKey: CustomerDataWriterKey
  mode: CustomerDataWriterMode
  watermark?: string
  sourceChecksum?: string
  targetChecksum?: string
  approvedBy?: string
  approvedAt?: string
  revision: number
  updatedAt?: string
}

export interface CustomerDataControlState {
  clientAccountId: string
  capabilities: CustomerDataCapabilityState[]
  writerStates: CustomerDataWriterState[]
}

export interface CustomerDataCapabilityWriteInput {
  mode: CustomerDataCapabilityMode
  expectedRevision: number
  idempotencyKey: string
  reason: string
}

export interface CustomerDataWriterWriteInput {
  mode: CustomerDataWriterMode
  watermark?: string
  sourceChecksum?: string
  targetChecksum?: string
  expectedRevision: number
  idempotencyKey: string
  reason: string
}

export const CUSTOMER_INTELLIGENCE_INVARIANTS = [
  'Modulo, tenant, RBAC e client scope nunca sao controlados por prompt.',
  'Consentimento, finalidade, allowlists, schema, retention e hard caps vencem configuracao.',
  'Runtime produz decisoes ou drafts; envio passa pelo Omnichannel, FSM e outbox.',
  'Fonte, tool, modelo e process key precisam existir em catalogo server-side.',
  'Segredos permanecem write-only e nunca retornam ao painel.',
] as const
