// Contrato de dados do modulo Finance (planilhas mensais de entradas/saidas).
//
// FONTE ATUAL: mock BFF temporario (web/server/api/admin/finance-*). Estes tipos
// espelham 1:1 o schema-alvo do back Go `finance.*` desenhado em
// docs/finance/PLANO_MODULO_FINANCE.md. Quando o back real entrar, o contrato
// permanece; muda apenas a origem (BFF -> API Go).

export interface FinanceLineAdjustment {
  id: string
  amount: number
  note: string
  date: string
}

export interface FinanceLineItem {
  id: string
  kind?: 'entrada' | 'saida'
  description: string
  category: string
  effective: boolean
  effectiveDate: string
  amount: number
  adjustmentAmount: number
  adjustments: FinanceLineAdjustment[]
  fixedAccountId: string
  details: string
}

export interface FinanceSheetSummary {
  expectedIn: number
  effectiveIn: number
  expectedOut: number
  effectiveOut: number
  expectedBalance: number
  effectiveBalance: number
}

export interface FinanceSheetListItem {
  id: string
  title: string
  period: string
  status: string
  notes: string
  coreTenantId?: string
  clientName: string
  summary: FinanceSheetSummary
  preview: string
  createdAt: string
  updatedAt: string
}

export interface FinanceSheetItem extends FinanceSheetListItem {
  entradas: FinanceLineItem[]
  saidas: FinanceLineItem[]
}

export interface FinancesListMeta {
  page: number
  limit: number
  total: number
  totalPages: number
  hasMore: boolean
}

export interface FinancesListResponse {
  status: 'success'
  data: FinanceSheetListItem[]
  meta: FinancesListMeta
}

export interface FinanceMutationResponse {
  status: 'success'
  data: FinanceSheetItem
}

export interface FinanceDetailResponse {
  status: 'success'
  data: FinanceSheetItem
}

export interface FinanceLineMutationData {
  sheetId: string
  lineId: string
  line: FinanceLineItem
  summary: FinanceSheetSummary
  preview: string
  updatedAt: string
}

export interface FinanceLineMutationResponse {
  status: 'success'
  data: FinanceLineMutationData
}

export type FinanceConfigKind = 'entrada' | 'saida' | 'ambas'

export interface FinanceCategoryConfig {
  id: string
  name: string
  kind: FinanceConfigKind
  description: string
}

export interface FinanceFixedAccountMember {
  id: string
  name: string
  amount: number
}

export interface FinanceFixedAccountConfig {
  id: string
  name: string
  kind: FinanceConfigKind
  categoryId: string
  defaultAmount: number
  notes: string
  members: FinanceFixedAccountMember[]
}

export interface FinanceConfigData {
  coreTenantId?: string
  categories: FinanceCategoryConfig[]
  fixedAccounts: FinanceFixedAccountConfig[]
  recurringEntries: FinanceRecurringEntryConfig[]
  updatedAt: string
}

export interface FinanceConfigResponse {
  status: 'success'
  data: FinanceConfigData
}

export interface FinanceRecurringEntryConfig {
  sourceCoreTenantId?: string
  adjustmentAmount: number
  notes: string
}

export interface FinanceRecurringClientStore {
  id: string
  name: string
  amount: number
}

export interface FinanceRecurringClientEntry {
  id: string
  coreTenantId: string
  name: string
  amount: number
  dueDay: string
  billingMode: 'single' | 'per_store'
  stores: FinanceRecurringClientStore[]
}
