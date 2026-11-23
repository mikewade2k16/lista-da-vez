// Normaliza o path de uma chamada de API para virar agrupador estavel: remove
// origem, querystring e hash e troca ids (numericos, uuid, hex longos) por `:id`.

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const LONG_HEX_PATTERN = /^[0-9a-f]{24,}$/i
const NUMERIC_PATTERN = /^\d+$/

function isIdSegment(segment: string): boolean {
  return (
    NUMERIC_PATTERN.test(segment) || UUID_PATTERN.test(segment) || LONG_HEX_PATTERN.test(segment)
  )
}

export function normalizeApiPath(rawPath: string): string {
  let path = String(rawPath || '')
  path = path.replace(/^[a-z][a-z\d+\-.]*:\/\/[^/?#]*/i, '')
  path = path.split('#')[0]!.split('?')[0]!
  if (!path.startsWith('/')) path = `/${path}`

  return path
    .split('/')
    .map((segment) => (segment && isIdSegment(segment) ? ':id' : segment))
    .join('/')
}

export function isPublicApiPath(rawPath: string): boolean {
  return normalizeApiPath(rawPath).startsWith('/v1/public/')
}

export const COLLECT_PATH = '/v1/obs/collect'

export function isCollectorPath(rawPath: string): boolean {
  return normalizeApiPath(rawPath).startsWith(COLLECT_PATH)
}
