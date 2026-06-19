// Absolutiza caminhos de midia servidos pelo back (`/uploads/*`) para a apiBase.
// O painel roda em outro dominio/porta que o back (ex.: web :3003 x api :9091),
// entao um `/uploads/...` relativo cairia no host errado e a imagem quebra. URLs
// absolutas (http/https/data) e `/assets/*` (servidos pelo proprio front) passam
// direto. Mesmo padrao usado no modulo bio (BioMediaField/BioLivePreview).
export function resolveMediaUrl(url: string | null | undefined, apiBase: string): string {
  const value = String(url ?? '').trim()
  if (!value) {
    return ''
  }
  const base = String(apiBase ?? '').replace(/\/$/, '')
  return base && value.startsWith('/uploads/') ? base + value : value
}
