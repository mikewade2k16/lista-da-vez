// Contrato do modulo tools (encurtador + QR). camelCase = igual ao DTO do back
// (back/internal/modules/tools/model.go). Ver docs/tools/PLANO_MODULO_TOOLS.md.

export interface ShortLinkItem {
  id: string
  slug: string
  targetUrl: string
  shortUrl: string
  hits: number
  createdAt: string
  accountId: string
  clientName: string
}

export interface QrCodeItem {
  id: string
  slug: string
  targetUrl: string
  qrUrl: string
  fillColor: string
  backColor: string
  size: number
  isActive: boolean
  scanCount: number
  lastScannedAt: string
  createdAt: string
  accountId: string
  clientName: string
}

export interface ToolsListMeta {
  page: number
  limit: number
  total: number
  totalPages: number
  hasMore: boolean
}

export interface ShortLinksListResponse {
  status: string
  data: ShortLinkItem[]
  meta: ToolsListMeta
}

export interface ShortLinkMutationResponse {
  status: string
  data: ShortLinkItem
}

export interface QrCodesListResponse {
  status: string
  data: QrCodeItem[]
  meta: ToolsListMeta
}

export interface QrCodeMutationResponse {
  status: string
  data: QrCodeItem
}

// Payloads de create/patch. Campos ausentes NAO sao enviados (o back usa
// DisallowUnknownFields: enviar campo desconhecido derruba a request).
export interface CreateShortLinkPayload {
  targetUrl: string
  slug?: string
  accountId?: string
}

export interface CreateQrCodePayload {
  targetUrl: string
  slug?: string
  fillColor?: string
  backColor?: string
  size?: number
  isActive?: boolean
  accountId?: string
}

export interface UpdateQrCodePayload {
  targetUrl?: string
  slug?: string
  fillColor?: string
  backColor?: string
  size?: number
  isActive?: boolean
}
