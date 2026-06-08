import type { AccountFieldKey, AccountItem } from '~/types/accounts'

// Fonte UNICA de definicao dos campos de account. Modal de detalhe e board card
// consomem a MESMA lista — assim "modal e board card ficam espelhados" por
// construcao: incluir/alterar um campo aqui reflete nos dois automaticamente
// (regra de UX do produto, ENGINEERING_PRINCIPLES secao 4).

export type AccountEditType = 'text' | 'switch' | 'select' | 'money' | 'number'

export type AccountFieldGroupId =
  | 'identidade'
  | 'billing'
  | 'contato'
  | 'webhook'
  | 'flags'
  | 'metrica'

export interface AccountFieldEdit {
  field: AccountFieldKey
  type: AccountEditType
  options?: { label: string; value: string }[]
  immediate?: boolean
}

export interface AccountFieldDef {
  key: string
  label: string
  group: AccountFieldGroupId
  // Valor formatado para leitura (card + resumo do modal).
  display: (account: AccountItem) => string
  // Quando presente, o campo e editavel no modal de detalhe.
  edit?: AccountFieldEdit
}

export interface AccountFieldGroupDef {
  id: AccountFieldGroupId
  label: string
}

export const ACCOUNT_FIELD_GROUPS: AccountFieldGroupDef[] = [
  { id: 'identidade', label: 'Identidade' },
  { id: 'billing', label: 'Cobranca' },
  { id: 'contato', label: 'Contato' },
  { id: 'webhook', label: 'Webhook' },
  { id: 'flags', label: 'Regras de cadastro' },
  { id: 'metrica', label: 'Metricas' },
]

const BILLING_MODE_OPTIONS = [
  { label: 'Unico', value: 'single' },
  { label: 'Por loja', value: 'per_store' },
]

function yesNo(value: boolean): string {
  return value ? 'Sim' : 'Nao'
}

function billingModeLabel(value: string): string {
  return value === 'per_store' ? 'Por loja' : 'Unico'
}

function moneyLabel(value: number): string {
  return `R$ ${(Number(value) || 0).toFixed(2)}`
}

function activeModuleNames(account: AccountItem): string {
  const names = (account.modules ?? [])
    .filter((m) => m.status === 'active')
    .map((m) => m.name || m.code)
  return names.length ? names.join(', ') : 'Sem modulos'
}

export const ACCOUNT_FIELDS: AccountFieldDef[] = [
  {
    key: 'name',
    label: 'Nome',
    group: 'identidade',
    display: (a) => a.name || '-',
    edit: { field: 'name', type: 'text' },
  },
  {
    key: 'slug',
    label: 'Slug',
    group: 'identidade',
    display: (a) => a.slug || '-',
    edit: { field: 'slug', type: 'text' },
  },
  {
    key: 'status',
    label: 'Ativo',
    group: 'identidade',
    display: (a) => (a.status === 'active' ? 'Ativo' : 'Inativo'),
    edit: { field: 'status', type: 'switch', immediate: true },
  },
  {
    key: 'organizationId',
    label: 'Organization',
    group: 'identidade',
    display: (a) => a.organizationId || 'Sem organization',
  },
  {
    key: 'billingMode',
    label: 'Modo cobranca',
    group: 'billing',
    display: (a) => billingModeLabel(a.billingMode),
    edit: { field: 'billingMode', type: 'select', options: BILLING_MODE_OPTIONS, immediate: true },
  },
  {
    key: 'monthlyPaymentAmount',
    label: 'Valor mensal',
    group: 'billing',
    display: (a) => moneyLabel(a.monthlyPaymentAmount),
    edit: { field: 'monthlyPaymentAmount', type: 'money' },
  },
  {
    key: 'paymentDueDay',
    label: 'Dia pagamento',
    group: 'billing',
    display: (a) => (a.paymentDueDay ? String(a.paymentDueDay) : '-'),
    edit: { field: 'paymentDueDay', type: 'number' },
  },
  {
    key: 'contactPhone',
    label: 'Telefone',
    group: 'contato',
    display: (a) => a.contactPhone || '-',
    edit: { field: 'contactPhone', type: 'text' },
  },
  {
    key: 'contactSite',
    label: 'Site',
    group: 'contato',
    display: (a) => a.contactSite || '-',
    edit: { field: 'contactSite', type: 'text' },
  },
  {
    key: 'contactAddress',
    label: 'Endereco',
    group: 'contato',
    display: (a) => a.contactAddress || '-',
    edit: { field: 'contactAddress', type: 'text' },
  },
  {
    key: 'webhookEnabled',
    label: 'Webhook ativo',
    group: 'webhook',
    display: (a) => yesNo(a.webhookEnabled),
    edit: { field: 'webhookEnabled', type: 'switch', immediate: true },
  },
  {
    key: 'webhookKey',
    label: 'Chave webhook',
    group: 'webhook',
    display: (a) => a.webhookKey || '-',
  },
  {
    key: 'requireUserStoreLink',
    label: 'Obriga loja',
    group: 'flags',
    display: (a) => yesNo(a.requireUserStoreLink),
    edit: { field: 'requireUserStoreLink', type: 'switch', immediate: true },
  },
  {
    key: 'requireUserRegistration',
    label: 'Obriga matricula',
    group: 'flags',
    display: (a) => yesNo(a.requireUserRegistration),
    edit: { field: 'requireUserRegistration', type: 'switch', immediate: true },
  },
  {
    key: 'userCount',
    label: 'Usuarios',
    group: 'metrica',
    display: (a) => String(a.userCount ?? 0),
  },
  {
    key: 'projectCount',
    label: 'Projetos',
    group: 'metrica',
    display: (a) => String(a.projectCount ?? 0),
  },
  { key: 'modules', label: 'Modulos', group: 'metrica', display: activeModuleNames },
]

// Subconjunto exibido no board card (resumo compacto). Mesmo conjunto que o
// modal destaca no topo — manter alinhado para preservar o espelho.
export const ACCOUNT_CARD_FIELD_KEYS = [
  'slug',
  'billingMode',
  'monthlyPaymentAmount',
  'userCount',
  'modules',
] as const

export function accountFieldsByGroup(groupId: AccountFieldGroupId): AccountFieldDef[] {
  return ACCOUNT_FIELDS.filter((f) => f.group === groupId)
}

export function accountCardFields(): AccountFieldDef[] {
  return ACCOUNT_CARD_FIELD_KEYS.map((key) => ACCOUNT_FIELDS.find((f) => f.key === key)).filter(
    (f): f is AccountFieldDef => Boolean(f),
  )
}
