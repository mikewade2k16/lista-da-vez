// ============================================================================
// MOCK BFF — Store in-memory do modulo Finance (TEMPORARIO)
// ============================================================================
// Este arquivo NAO e a fonte de verdade. Ele existe apenas para deixar a tela
// /finance clicavel enquanto o back Go real (schema finance.*) nao entra.
//
//   - Dados vivem em memoria do processo Nitro: somem a cada restart do dev.
//   - So funciona em dev/SSR (nuxt dev). Em `nuxt generate` estatico nao ha server.
//   - Alvo de remocao: substituir por back/internal/modules/finance/ + API Go.
//     Ver docs/finance/PLANO_MODULO_FINANCE.md e docs/LEGADO.md.
//
// Registrado em docs/LEGADO.md. Nao evoluir regra de negocio aqui — o minimo
// para exercitar a UI (autosave, efetivacao, recorrencia, configuracao).
import { randomUUID } from 'node:crypto'

export interface MockAdjustment {
  id: string
  amount: number
  note: string
  date: string
}

export interface MockLine {
  id: string
  kind?: 'entrada' | 'saida'
  description: string
  category: string
  effective: boolean
  effectiveDate: string
  amount: number
  adjustmentAmount: number
  adjustments: MockAdjustment[]
  fixedAccountId: string
  details: string
}

export interface MockSummary {
  expectedIn: number
  effectiveIn: number
  expectedOut: number
  effectiveOut: number
  expectedBalance: number
  effectiveBalance: number
}

export interface MockSheet {
  id: string
  title: string
  period: string
  status: string
  notes: string
  coreTenantId: string
  clientName: string
  entradas: MockLine[]
  saidas: MockLine[]
  createdAt: string
  updatedAt: string
}

interface MockCategory {
  id: string
  name: string
  kind: 'entrada' | 'saida' | 'ambas'
  description: string
}

interface MockFixedMember {
  id: string
  name: string
  amount: number
}

interface MockFixedAccount {
  id: string
  name: string
  kind: 'entrada' | 'saida' | 'ambas'
  categoryId: string
  defaultAmount: number
  notes: string
  members: MockFixedMember[]
}

interface MockRecurringEntry {
  sourceCoreTenantId?: string
  adjustmentAmount: number
  notes: string
}

interface MockConfig {
  coreTenantId: string
  categories: MockCategory[]
  fixedAccounts: MockFixedAccount[]
  recurringEntries: MockRecurringEntry[]
  updatedAt: string
}

// Chave de escopo: coreTenantId (cliente). '' = escopo padrao/global do mock.
const sheetsByScope = new Map<string, MockSheet[]>()
const configByScope = new Map<string, MockConfig>()
let seeded = false

function nowIso() {
  return new Date().toISOString()
}

function currentPeriod() {
  const now = new Date()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  return `${now.getFullYear()}-${month}`
}

export function scopeKey(coreTenantId: unknown) {
  return String(coreTenantId ?? '')
    .trim()
    .toLowerCase()
}

function num(value: unknown, allowNegative = false) {
  const parsed = Number(value ?? 0)
  if (!Number.isFinite(parsed)) return 0
  const rounded = Number(parsed.toFixed(2))
  return allowNegative ? rounded : Math.max(0, rounded)
}

function text(value: unknown, max = 12000) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function normalizeLine(raw: Partial<MockLine> | undefined): MockLine {
  const adjustments = Array.isArray(raw?.adjustments)
    ? raw!.adjustments.map((a) => ({
        id: String(a?.id || randomUUID()),
        amount: num(a?.amount, true),
        note: text(a?.note, 240),
        date: text(a?.date, 10),
      }))
    : []
  const adjustmentAmount =
    adjustments.length > 0
      ? Number(adjustments.reduce((sum, a) => sum + a.amount, 0).toFixed(2))
      : num(raw?.adjustmentAmount, true)

  return {
    id: String(raw?.id || randomUUID()),
    description: text(raw?.description, 260),
    category: text(raw?.category, 120),
    effective: Boolean(raw?.effective),
    effectiveDate: text(raw?.effectiveDate, 10),
    amount: num(raw?.amount, false),
    adjustmentAmount,
    adjustments,
    fixedAccountId: text(raw?.fixedAccountId, 90),
    details: text(raw?.details, 600),
  }
}

export function computeSummary(sheet: Pick<MockSheet, 'entradas' | 'saidas'>): MockSummary {
  const lineTotal = (line: MockLine) =>
    Number(line.amount || 0) + Number(line.adjustmentAmount || 0)
  const expectedIn = sheet.entradas.reduce((s, l) => s + lineTotal(l), 0)
  const effectiveIn = sheet.entradas.reduce((s, l) => (l.effective ? s + lineTotal(l) : s), 0)
  const expectedOut = sheet.saidas.reduce((s, l) => s + lineTotal(l), 0)
  const effectiveOut = sheet.saidas.reduce((s, l) => (l.effective ? s + lineTotal(l) : s), 0)
  return {
    expectedIn: Number(expectedIn.toFixed(2)),
    effectiveIn: Number(effectiveIn.toFixed(2)),
    expectedOut: Number(expectedOut.toFixed(2)),
    effectiveOut: Number(effectiveOut.toFixed(2)),
    expectedBalance: Number((expectedIn - expectedOut).toFixed(2)),
    effectiveBalance: Number((effectiveIn - effectiveOut).toFixed(2)),
  }
}

function computePreview(summary: MockSummary) {
  return `Entradas ${summary.effectiveIn} | Saidas ${summary.effectiveOut} | Saldo ${summary.effectiveBalance}`
}

export function toListItem(sheet: MockSheet) {
  const summary = computeSummary(sheet)
  return {
    id: sheet.id,
    title: sheet.title,
    period: sheet.period,
    status: sheet.status,
    notes: sheet.notes,
    coreTenantId: sheet.coreTenantId,
    clientName: sheet.clientName,
    summary,
    preview: computePreview(summary),
    createdAt: sheet.createdAt,
    updatedAt: sheet.updatedAt,
  }
}

export function toDetail(sheet: MockSheet) {
  return {
    ...toListItem(sheet),
    entradas: sheet.entradas,
    saidas: sheet.saidas,
  }
}

function ensureSeed() {
  if (seeded) return
  seeded = true

  const scope = ''
  const receitaId = randomUUID()
  const custoId = randomUUID()
  configByScope.set(scope, {
    coreTenantId: scope,
    categories: [
      { id: receitaId, name: 'Receita', kind: 'entrada', description: 'Entradas do mes' },
      { id: custoId, name: 'Custos fixos', kind: 'saida', description: 'Saidas recorrentes' },
    ],
    fixedAccounts: [
      {
        id: randomUUID(),
        name: 'Folha salarial',
        kind: 'saida',
        categoryId: custoId,
        defaultAmount: 3500,
        notes: 'Equipe fixa',
        members: [
          { id: randomUUID(), name: 'Colaborador 1', amount: 2000 },
          { id: randomUUID(), name: 'Colaborador 2', amount: 1500 },
        ],
      },
    ],
    recurringEntries: [],
    updatedAt: nowIso(),
  })

  const sheet: MockSheet = {
    id: randomUUID(),
    title: `Finance ${currentPeriod()}`,
    period: currentPeriod(),
    status: 'aberta',
    notes: '',
    coreTenantId: scope,
    clientName: 'Cliente demo (mock)',
    entradas: [
      normalizeLine({
        description: 'Servicos',
        category: 'Receita',
        amount: 8000,
        effective: true,
        effectiveDate: '',
      }),
    ],
    saidas: [
      normalizeLine({
        description: 'Aluguel',
        category: 'Custos fixos',
        amount: 2200,
        effective: false,
      }),
    ],
    createdAt: nowIso(),
    updatedAt: nowIso(),
  }
  sheetsByScope.set(scope, [sheet])
}

function sheetsFor(scope: string) {
  ensureSeed()
  if (!sheetsByScope.has(scope)) sheetsByScope.set(scope, [])
  return sheetsByScope.get(scope)!
}

export function listSheets(coreTenantId: unknown, opts: { q?: string; period?: string } = {}) {
  const scope = scopeKey(coreTenantId)
  const q = text(opts.q, 120).toLowerCase()
  const period = text(opts.period, 7)
  let list = sheetsFor(scope).slice()
  if (period) list = list.filter((s) => s.period === period)
  if (q) list = list.filter((s) => `${s.title} ${s.clientName}`.toLowerCase().includes(q))
  return list.sort((a, b) => (a.updatedAt < b.updatedAt ? 1 : -1)).map(toListItem)
}

function findSheet(id: string) {
  ensureSeed()
  for (const [, list] of sheetsByScope) {
    const found = list.find((s) => s.id === id)
    if (found) return found
  }
  return null
}

export function getSheet(id: string) {
  const sheet = findSheet(id)
  return sheet ? toDetail(sheet) : null
}

export function createSheet(body: Record<string, unknown>) {
  const scope = scopeKey(body.coreTenantId)
  const sheet: MockSheet = {
    id: randomUUID(),
    title: text(body.title, 180) || `Finance ${currentPeriod()}`,
    period: text(body.period, 7) || currentPeriod(),
    status: text(body.status, 120) || 'aberta',
    notes: text(body.notes, 12000),
    coreTenantId: scope,
    clientName: 'Cliente demo (mock)',
    entradas: Array.isArray(body.entradas)
      ? (body.entradas as MockLine[]).map((l) => normalizeLine(l))
      : [],
    saidas: Array.isArray(body.saidas)
      ? (body.saidas as MockLine[]).map((l) => normalizeLine(l))
      : [],
    createdAt: nowIso(),
    updatedAt: nowIso(),
  }
  sheetsFor(scope).unshift(sheet)
  return toDetail(sheet)
}

export function updateSheet(id: string, body: Record<string, unknown>) {
  const sheet = findSheet(id)
  if (!sheet) return null
  sheet.title = text(body.title, 180)
  sheet.period = text(body.period, 7) || sheet.period
  sheet.status = text(body.status, 120)
  sheet.notes = text(body.notes, 12000)
  if (Array.isArray(body.entradas))
    sheet.entradas = (body.entradas as MockLine[]).map((l) => normalizeLine(l))
  if (Array.isArray(body.saidas))
    sheet.saidas = (body.saidas as MockLine[]).map((l) => normalizeLine(l))
  sheet.updatedAt = nowIso()
  return toDetail(sheet)
}

export function deleteSheet(id: string) {
  ensureSeed()
  for (const [, list] of sheetsByScope) {
    const index = list.findIndex((s) => s.id === id)
    if (index >= 0) {
      list.splice(index, 1)
      return true
    }
  }
  return false
}

export function patchLine(sheetId: string, lineId: string, body: Record<string, unknown>) {
  const sheet = findSheet(sheetId)
  if (!sheet) return null
  const line =
    sheet.entradas.find((l) => l.id === lineId) || sheet.saidas.find((l) => l.id === lineId)
  if (!line) return null

  if (Object.prototype.hasOwnProperty.call(body, 'effective')) {
    line.effective = Boolean(body.effective)
    if (!line.effective) line.effectiveDate = ''
  }
  if (Object.prototype.hasOwnProperty.call(body, 'effectiveDate')) {
    line.effectiveDate = text(body.effectiveDate, 10)
  }
  sheet.updatedAt = nowIso()

  const summary = computeSummary(sheet)
  return {
    sheetId: sheet.id,
    lineId: line.id,
    line,
    summary,
    preview: computePreview(summary),
    updatedAt: sheet.updatedAt,
  }
}

function emptyConfig(scope: string): MockConfig {
  return {
    coreTenantId: scope,
    categories: [],
    fixedAccounts: [],
    recurringEntries: [],
    updatedAt: nowIso(),
  }
}

export function getConfig(coreTenantId: unknown) {
  ensureSeed()
  const scope = scopeKey(coreTenantId)
  return configByScope.get(scope) || emptyConfig(scope)
}

export function saveConfig(body: Record<string, unknown>) {
  const scope = scopeKey(body.coreTenantId)
  const next: MockConfig = {
    coreTenantId: scope,
    categories: Array.isArray(body.categories)
      ? (body.categories as MockCategory[]).map((c) => ({
          id: String(c.id || randomUUID()),
          name: text(c.name, 120),
          kind: c.kind === 'entrada' || c.kind === 'saida' ? c.kind : 'ambas',
          description: text(c.description, 400),
        }))
      : [],
    fixedAccounts: Array.isArray(body.fixedAccounts)
      ? (body.fixedAccounts as MockFixedAccount[]).map((a) => ({
          id: String(a.id || randomUUID()),
          name: text(a.name, 120),
          kind: a.kind === 'entrada' || a.kind === 'saida' ? a.kind : 'ambas',
          categoryId: text(a.categoryId, 90),
          defaultAmount: num(a.defaultAmount),
          notes: text(a.notes, 500),
          members: Array.isArray(a.members)
            ? a.members.map((m) => ({
                id: String(m.id || randomUUID()),
                name: text(m.name, 120),
                amount: num(m.amount),
              }))
            : [],
        }))
      : [],
    recurringEntries: Array.isArray(body.recurringEntries)
      ? (body.recurringEntries as MockRecurringEntry[]).map((e) => ({
          sourceCoreTenantId: text(e.sourceCoreTenantId, 90) || undefined,
          adjustmentAmount: num(e.adjustmentAmount, true),
          notes: text(e.notes, 240),
        }))
      : [],
    updatedAt: nowIso(),
  }
  configByScope.set(scope, next)
  return next
}

// Recorrencias de clientes: no back real vira read model (join core.accounts +
// finance.recurring_entries). No mock, retorna vazio por padrao.
export function listRecurringClients(_coreTenantId: unknown) {
  return [] as Array<{
    id: string
    coreTenantId: string
    name: string
    monthlyPaymentAmount: number
    paymentDueDay: string
    billingMode: 'single' | 'per_store'
    stores: Array<{ id: string; name: string; amount: number }>
  }>
}
