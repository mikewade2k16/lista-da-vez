export interface ServerLead {
  id: number
  clientId: number
  sourceLeadId: number | null
  source: string
  name: string
  email: string
  phone: string
  page: string
  cupom: string
  consent: number
  ip: string
  device: string
  userAgent: string
  trackingData: string
  payloadJson: string
  createdAt: string
}

export interface LeadsListMeta {
  page: number
  limit: number
  total: number
  totalPages: number
  hasMore: boolean
}

export interface LeadListOptions {
  page: number
  limit: number
  q?: string
  clientId?: string | number
  viewerUserType?: 'admin' | 'client'
  viewerClientId?: number
}

export interface LeadAccessOptions {
  viewerUserType?: 'admin' | 'client'
  viewerClientId?: number
}

const globalKey = '__omni_leads_repo__'

interface RepositoryState {
  leads: ServerLead[]
  nextId: number
}

const CLIENT_NAME_MAP: Record<number, string> = {
  103: 'Crow',
  104: 'Dr Antonio Tavares',
  105: 'Perola',
  106: 'crow',
}

function nowIso() {
  return new Date().toISOString()
}

function normalizeText(value: unknown, max = 255) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function normalizeClientId(value: unknown) {
  const parsed = Number.parseInt(String(value ?? ''), 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }

  return parsed
}

function canAccessLead(lead: ServerLead, options?: LeadAccessOptions) {
  const viewerType = String(options?.viewerUserType ?? 'admin')
    .trim()
    .toLowerCase()
  if (viewerType !== 'client') {
    return true
  }

  const viewerClientId = normalizeClientId(options?.viewerClientId)
  if (viewerClientId <= 0) {
    return false
  }

  return lead.clientId === viewerClientId
}

function normalizeSearch(value: unknown) {
  return String(value ?? '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }

  const day = String(date.getUTCDate()).padStart(2, '0')
  const month = String(date.getUTCMonth() + 1).padStart(2, '0')
  const year = String(date.getUTCFullYear())
  const hours = String(date.getUTCHours()).padStart(2, '0')
  const minutes = String(date.getUTCMinutes()).padStart(2, '0')
  return `${day}/${month}/${year} ${hours}:${minutes}`
}

function resolveClientName(clientId: number) {
  return CLIENT_NAME_MAP[clientId] || `Cliente #${clientId || 0}`
}

function parseJsonToString(value: string) {
  const raw = String(value ?? '').trim()
  if (!raw) {
    return ''
  }

  try {
    const parsed = JSON.parse(raw)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return raw
  }
}

function buildFallbackPayload(lead: ServerLead) {
  const payload: Record<string, unknown> = {
    nome: normalizeText(lead.name, 255),
    email: normalizeText(lead.email, 255),
    telefone: normalizeText(lead.phone, 60),
    origem: normalizeText(lead.source, 120),
    pagina: normalizeText(lead.page, 150),
    cupom: normalizeText(lead.cupom, 80),
    consentimento: lead.consent === 1 ? 'Sim' : 'Nao',
    cliente: resolveClientName(lead.clientId),
  }

  const trackingString = parseJsonToString(lead.trackingData)
  if (trackingString) {
    payload.tracking_data = trackingString
  }

  return JSON.stringify(payload, null, 2)
}

function toDto(lead: ServerLead) {
  const payloadRaw = String(lead.payloadJson ?? '').trim()

  return {
    id: lead.id,
    clientId: lead.clientId,
    sourceLeadId: lead.sourceLeadId,
    source: normalizeText(lead.source, 120),
    nome: normalizeText(lead.name, 255),
    email: normalizeText(lead.email, 255),
    telefone: normalizeText(lead.phone, 60),
    page: normalizeText(lead.page, 150),
    cupom: normalizeText(lead.cupom, 80),
    consent: lead.consent === 1,
    consentLabel: lead.consent === 1 ? 'Sim' : 'Nao',
    trackingData: parseJsonToString(lead.trackingData),
    payloadJson: payloadRaw ? parseJsonToString(payloadRaw) : buildFallbackPayload(lead),
    formattedDate: formatDateTime(lead.createdAt),
    createdAt: lead.createdAt,
    clientName: resolveClientName(lead.clientId),
  }
}

function seedLeads(): ServerLead[] {
  return [
    {
      id: 2011,
      clientId: 105,
      sourceLeadId: 90001,
      source: 'webhook',
      name: 'leads Webhook 1',
      email: 'mikewade2k16@gmail.com',
      phone: '(79) 9 9975-4835',
      page: 'Home',
      cupom: '',
      consent: 1,
      ip: '177.22.10.31',
      device: 'mobile',
      userAgent: 'Mozilla/5.0',
      trackingData: '{"utm_source":"meta","utm_campaign":"leads_perola_home"}',
      payloadJson: '',
      createdAt: '2026-02-03T09:52:00.000Z',
    },
    {
      id: 2012,
      clientId: 105,
      sourceLeadId: 90002,
      source: 'landing',
      name: 'Carolina Sales',
      email: 'carol.sales@teste.com',
      phone: '(11) 97777-1002',
      page: 'Cupom Valentine',
      cupom: 'LOVE2026',
      consent: 1,
      ip: '177.10.10.88',
      device: 'desktop',
      userAgent: 'Mozilla/5.0',
      trackingData: '{"utm_source":"google","utm_medium":"cpc"}',
      payloadJson: '',
      createdAt: '2026-02-05T14:31:00.000Z',
    },
    {
      id: 2013,
      clientId: 104,
      sourceLeadId: 90003,
      source: 'form',
      name: 'Antonio Leads',
      email: 'antonio.leads@teste.com',
      phone: '(85) 98888-2010',
      page: 'Consulta',
      cupom: '',
      consent: 0,
      ip: '177.20.11.43',
      device: 'mobile',
      userAgent: 'Mozilla/5.0',
      trackingData: '{"utm_source":"organic"}',
      payloadJson: '',
      createdAt: '2026-02-08T11:12:00.000Z',
    },
    {
      id: 2014,
      clientId: 103,
      sourceLeadId: 90004,
      source: 'webhook',
      name: 'Crow Leads 1',
      email: 'crow.leads@teste.com',
      phone: '(21) 96666-3003',
      page: 'Contato',
      cupom: 'CROWVIP',
      consent: 1,
      ip: '179.20.10.77',
      device: 'desktop',
      userAgent: 'Mozilla/5.0',
      trackingData: '{"utm_source":"instagram"}',
      payloadJson: '',
      createdAt: '2026-02-10T16:05:00.000Z',
    },
  ]
}

function getState(): RepositoryState {
  const globalRef = globalThis as typeof globalThis & { [globalKey]?: RepositoryState }
  if (!globalRef[globalKey]) {
    const seeded = seedLeads().sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1))
    globalRef[globalKey] = {
      leads: seeded,
      nextId: Math.max(...seeded.map((item) => item.id)) + 1,
    }
  }

  return globalRef[globalKey] as RepositoryState
}

export function listLeads(options: LeadListOptions) {
  const state = getState()
  const page = Number.isFinite(options.page) && options.page > 0 ? options.page : 1
  const limit =
    Number.isFinite(options.limit) && options.limit > 0 ? Math.min(options.limit, 300) : 50
  const q = normalizeSearch(options.q)
  const clientId = normalizeClientId(options.clientId)

  const filtered = state.leads.filter((lead) => {
    if (!canAccessLead(lead, options)) {
      return false
    }
    if (clientId > 0 && lead.clientId !== clientId) {
      return false
    }
    if (!q) {
      return true
    }

    const haystack = normalizeSearch(
      [
        lead.name,
        lead.email,
        lead.phone,
        lead.source,
        lead.page,
        lead.cupom,
        resolveClientName(lead.clientId),
        lead.payloadJson,
        lead.trackingData,
      ].join(' '),
    )

    return haystack.includes(q)
  })

  const sorted = [...filtered].sort((a, b) => {
    if (a.createdAt === b.createdAt) {
      return b.id - a.id
    }

    return a.createdAt < b.createdAt ? 1 : -1
  })

  const total = sorted.length
  const totalPages = Math.max(1, Math.ceil(total / limit))
  const safePage = Math.min(page, totalPages)
  const start = (safePage - 1) * limit
  const items = sorted.slice(start, start + limit).map(toDto)

  const clients = Array.from(
    new Set(
      state.leads.filter((lead) => canAccessLead(lead, options)).map((item) => item.clientId),
    ),
  )
    .filter((client) => client > 0)
    .sort((a, b) => a - b)
    .map((client) => ({ id: client, name: resolveClientName(client) }))

  return {
    items,
    meta: {
      page: safePage,
      limit,
      total,
      totalPages,
      hasMore: safePage < totalPages,
    } satisfies LeadsListMeta,
    filters: {
      clients,
    },
  }
}

export function deleteLeadById(id: number, options?: LeadAccessOptions) {
  const state = getState()
  const index = state.leads.findIndex((item) => item.id === id)
  if (index < 0) {
    return false
  }

  const target = state.leads[index]
  if (!target || !canAccessLead(target, options)) {
    return false
  }

  state.leads.splice(index, 1)
  return true
}

export function createLeadForTest(input: Partial<ServerLead> = {}, options?: LeadAccessOptions) {
  const state = getState()
  const id = state.nextId++
  const viewerType = String(options?.viewerUserType ?? 'admin')
    .trim()
    .toLowerCase()
  const viewerClientId = normalizeClientId(options?.viewerClientId)
  const requestedClientId = normalizeClientId(input.clientId)
  const resolvedClientId =
    viewerType === 'client'
      ? viewerClientId > 0
        ? viewerClientId
        : 105
      : requestedClientId > 0
        ? requestedClientId
        : viewerClientId > 0
          ? viewerClientId
          : 105

  const created: ServerLead = {
    id,
    clientId: resolvedClientId,
    sourceLeadId: input.sourceLeadId ?? null,
    source: normalizeText(input.source, 120) || 'manual',
    name: normalizeText(input.name, 255) || `Novo lead ${id}`,
    email: normalizeText(input.email, 255),
    phone: normalizeText(input.phone, 60),
    page: normalizeText(input.page, 150),
    cupom: normalizeText(input.cupom, 80),
    consent: Number(input.consent ?? 0) === 1 ? 1 : 0,
    ip: normalizeText(input.ip, 45),
    device: normalizeText(input.device, 30),
    userAgent: normalizeText(input.userAgent, 255),
    trackingData: normalizeText(input.trackingData, 2000),
    payloadJson: normalizeText(input.payloadJson, 4000),
    createdAt: nowIso(),
  }

  state.leads.unshift(created)
  return toDto(created)
}
