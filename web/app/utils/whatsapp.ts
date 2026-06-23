// Helpers de link do WhatsApp. Centraliza a normalizacao de telefone e a
// montagem do link wa.me, substituindo o `wa.me` inline solto que existia em
// componentes de leads. Sem efeito colateral no import.

const BRAZIL_DDI = '55'

// Numeros nacionais brasileiros tem 10 (fixo) ou 11 (celular) digitos sem DDI.
// Fora dessa faixa, assume-se que o DDI ja foi informado.
const MIN_NATIONAL_DIGITS = 10
const MAX_NATIONAL_DIGITS = 11

// Monta o link wa.me a partir de um telefone e um texto opcional.
// Normaliza para somente digitos; se o numero tiver 10 ou 11 digitos (formato
// nacional sem DDI), prefixa o DDI do Brasil (55). Retorna '' se o telefone for
// vazio ou nao tiver nenhum digito.
export function buildWhatsappLink(phone: string, text?: string): string {
  const digits = String(phone ?? '').replace(/\D+/g, '')
  if (!digits) return ''

  const hasNationalLength =
    digits.length >= MIN_NATIONAL_DIGITS && digits.length <= MAX_NATIONAL_DIGITS
  const normalized = hasNationalLength ? `${BRAZIL_DDI}${digits}` : digits

  const query = text ? `?text=${encodeURIComponent(text)}` : ''
  return `https://wa.me/${normalized}${query}`
}

// Abre o link do WhatsApp em uma nova aba (noopener,noreferrer). No-op quando o
// link nao pode ser montado (telefone vazio/sem digitos) ou fora do browser.
export function openWhatsapp(phone: string, text?: string): void {
  if (typeof window === 'undefined') return
  const link = buildWhatsappLink(phone, text)
  if (!link) return
  window.open(link, '_blank', 'noopener,noreferrer')
}
