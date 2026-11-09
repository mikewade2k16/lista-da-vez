// Producao usa o dominio cadastrado do estabelecimento aberto, nunca o Studio
// de outro cliente. Desenvolvimento pode usar o servidor local configurado.
export function resolveCardapioStudioUrl(
  domain: string,
  developmentUrl: string,
  development: boolean,
): string {
  if (development) return developmentUrl.trim().replace(/\/+$/, '')
  const host = domain.trim().toLowerCase()
  if (!host || /[\s/@?#\\]/.test(host)) return ''
  try {
    const url = new URL(`https://${host}`)
    return url.host === host && url.pathname === '/' ? url.origin : ''
  } catch {
    return ''
  }
}
