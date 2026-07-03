// Tipos e defaults do PERFIL ESTRATEGICO do cliente (contrato C3 do
// CALENDARIO_SPECS). Separado de calendar.ts/calendar-config.ts para manter cada
// arquivo < 450 linhas; o calendar.ts re-exporta daqui. SEM estado, SEM fetch.

/** Bloco `extra` (jsonb): campos livres de estrategia (todos strings). */
export interface CalendarClientProfileExtra {
  audience: string
  offer: string
  pillars: string
  cadence: string
  restrictions: string
  performance: string
  assets: string
}

/** Perfil estrategico de um cliente (contrato C3). PUT = upsert full-replace. */
export interface CalendarClientProfile {
  clientId: string
  segment: string
  positioning: string
  description: string
  history: string
  siteUrl: string
  instagram: string
  address: string
  objectives: string
  brandVoice: string
  extra: CalendarClientProfileExtra
  updatedAt: string
}

/** Uma entrada do index (`GET /v1/calendar/client-profiles`). */
export interface CalendarClientProfileIndexItem {
  clientId: string
  filled: boolean
  updatedAt: string
}

export function defaultClientProfileExtra(): CalendarClientProfileExtra {
  return {
    audience: '',
    offer: '',
    pillars: '',
    cadence: '',
    restrictions: '',
    performance: '',
    assets: '',
  }
}

export function defaultClientProfile(clientId = ''): CalendarClientProfile {
  return {
    clientId,
    segment: '',
    positioning: '',
    description: '',
    history: '',
    siteUrl: '',
    instagram: '',
    address: '',
    objectives: '',
    brandVoice: '',
    extra: defaultClientProfileExtra(),
    updatedAt: '',
  }
}

// Normaliza a resposta do back (perfil inexistente = objeto vazio com defaults,
// contrato C3: GET de perfil ausente devolve 200 + defaults, nunca 404). Merge por
// secao para nao perder o shape completo se o back omitir um campo novo.
export function normalizeClientProfile(res: unknown, clientId: string): CalendarClientProfile {
  const base = defaultClientProfile(clientId)
  const raw = (res as Partial<CalendarClientProfile>) || {}
  const rawExtra = (raw.extra as Partial<CalendarClientProfileExtra>) || {}
  return {
    clientId: typeof raw.clientId === 'string' && raw.clientId ? raw.clientId : clientId,
    segment: str(raw.segment),
    positioning: str(raw.positioning),
    description: str(raw.description),
    history: str(raw.history),
    siteUrl: str(raw.siteUrl),
    instagram: str(raw.instagram),
    address: str(raw.address),
    objectives: str(raw.objectives),
    brandVoice: str(raw.brandVoice),
    extra: { ...base.extra, ...pickStrings(rawExtra) },
    updatedAt: str(raw.updatedAt),
  }
}

function str(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

// So mantem chaves cujo valor e string (defesa contra shape inesperado do jsonb).
function pickStrings(obj: Record<string, unknown>): Partial<CalendarClientProfileExtra> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(obj)) {
    if (typeof value === 'string') out[key] = value
  }
  return out as Partial<CalendarClientProfileExtra>
}
