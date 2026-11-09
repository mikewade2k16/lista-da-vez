// Rótulos puros da curadoria do acervo público (testáveis sem DOM).
import type {
  PublicAttempt,
  PublicCatalogStatus,
  PublicReviewSummary,
  PublicVerdict,
} from '~/types/public-knowledge'

const ATTEMPT_ERRORS: Record<string, string> = {
  source_fetch_unavailable: 'o site não respondeu ou ficou fora do ar; tente mais tarde',
  source_http_rejected: 'o site recusou o download (bloqueio ou exige login); não contornamos',
  source_type_or_size_rejected: 'o arquivo mudou de formato ou passou do tamanho permitido',
  source_not_allowed: 'endereço fora das regras de segurança da coleta',
  source_dns_unavailable: 'não foi possível localizar o site (DNS)',
  extraction_incomplete: 'o PDF parece digitalizado: a maioria das páginas não tem texto',
  extraction_failed: 'não foi possível extrair o texto do arquivo',
  extraction_empty: 'o arquivo não tem texto aproveitável',
  extraction_unavailable: 'extrator de PDF indisponível no servidor',
  too_many_chunks: 'documento grande demais para uma coleta',
}

export function attemptLabel(attempt?: PublicAttempt): string {
  if (!attempt) return 'Nunca coletada'
  const when = formatPublicDate(attempt.createdAt)
  switch (attempt.outcome) {
    case 'collected':
      return `Coletada em ${when}`
    case 'unchanged':
      return `Conferida em ${when}: sem mudança desde a última coleta`
    default:
      return `Falhou em ${when}: ${ATTEMPT_ERRORS[attempt.errorCode ?? ''] ?? attempt.errorCode ?? 'erro desconhecido'}`
  }
}

const VERDICTS: Record<PublicVerdict | 'pending', string> = {
  approved: 'aprovado',
  changes_requested: 'pediu ajustes',
  rejected: 'reprovado',
  pending: 'pendente',
}

export function verdictLabel(verdict: PublicVerdict | 'pending'): string {
  return VERDICTS[verdict]
}

export function reviewSummaryLabel(review: PublicReviewSummary): string {
  const ai =
    review.aiApproved || review.aiConcerns
      ? `IA: ${review.aiApproved} aprovaram${review.aiConcerns ? `, ${review.aiConcerns} com ressalvas` : ''}`
      : 'IA: nenhuma revisão'
  return `Você: ${verdictLabel(review.owner)} · ${ai} · Contador: ${verdictLabel(review.accountant)}`
}

export function catalogStatusLabel(item: PublicCatalogStatus): string {
  const latest = item.latest
  if (latest?.status === 'collected') return `Versão ${latest.version} aguardando revisão`
  if (item.published) return `Publicada (versão ${item.published.version})`
  if (item.lastAttempt?.outcome === 'failed') return 'Falha na coleta'
  return 'Não publicada'
}

export function formatPublicDate(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('pt-BR', { dateStyle: 'short', timeStyle: 'short' }).format(date)
}

export function canPublish(review: PublicReviewSummary): boolean {
  return review.owner === 'approved'
}
