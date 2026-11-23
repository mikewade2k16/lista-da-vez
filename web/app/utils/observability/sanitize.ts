// Sanitizacao client-side de textos de erro antes de sair do navegador. O servidor
// tambem sanitiza, mas dado pessoal nao deve nem trafegar: mascara e-mails,
// CPF/CNPJ, sequencias longas de digitos (cartao/conta), tokens e querystrings.

export const MAX_TEXT_LENGTH = 2000

const JWT_PATTERN = /eyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*/g
const BEARER_PATTERN = /Bearer\s+[A-Za-z0-9._~+/=-]+/gi
const EMAIL_PATTERN = /[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}/g
const CNPJ_PATTERN = /\b\d{2}\.\d{3}\.\d{3}\/\d{4}-\d{2}\b/g
const CPF_PATTERN = /\b\d{3}\.\d{3}\.\d{3}-\d{2}\b/g
// 13 a 19 digitos, tolerando espacos/hifens entre grupos (numero de cartao).
const LONG_DIGITS_PATTERN = /\b\d(?:[ -]?\d){12,18}\b/g
const CPF_RAW_PATTERN = /\b\d{11}\b/g
const QUERY_PATTERN = /\?[^\s"'`)\]}>]*/g

export function sanitizeText(input: unknown, maxLength = MAX_TEXT_LENGTH): string {
  let text = typeof input === 'string' ? input : String(input ?? '')
  if (!text) return ''

  // Corta antes de aplicar as regex para limitar o custo em textos gigantes.
  text = text.slice(0, maxLength * 2)

  text = text
    .replace(BEARER_PATTERN, 'Bearer [token]')
    .replace(JWT_PATTERN, '[jwt]')
    .replace(EMAIL_PATTERN, '[email]')
    .replace(CNPJ_PATTERN, '[cnpj]')
    .replace(CPF_PATTERN, '[cpf]')
    .replace(LONG_DIGITS_PATTERN, '[num]')
    .replace(CPF_RAW_PATTERN, '[cpf]')
    .replace(QUERY_PATTERN, '?[query]')

  return text.slice(0, maxLength)
}
