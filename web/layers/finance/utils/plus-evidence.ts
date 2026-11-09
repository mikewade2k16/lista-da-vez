const REVIEW_VERDICTS: Record<string, string> = {
  approved: 'aprovado',
  changes_requested: 'pediu ajustes',
  rejected: 'reprovado',
  pending: 'pendente',
}

// Situação de revisão da fonte pública no momento da resposta (dono, IAs, contador).
export function evidenceReviewLabel(value: unknown): string {
  if (!value || typeof value !== 'object') return ''
  const review = value as Record<string, unknown>
  const owner = REVIEW_VERDICTS[String(review.owner)]
  const accountant = REVIEW_VERDICTS[String(review.accountant)]
  if (!owner || !accountant) return ''
  const ai =
    typeof review.aiApproved === 'number' && review.aiApproved > 0
      ? `${review.aiApproved} IA(s) aprovaram`
      : 'sem revisão de IA'
  return `Revisão: dono ${owner} · ${ai} · contador ${accountant}`
}

export function evidenceCards(values: unknown[]) {
  return values.flatMap((item) => {
    if (!item || typeof item !== 'object') return []
    const value = item as Record<string, unknown>
    if (
      typeof value.chunkId !== 'string' ||
      typeof value.excerpt !== 'string' ||
      typeof value.version !== 'number' ||
      !Number.isInteger(value.version) ||
      value.version < 1
    )
      return []
    const source = typeof value.sourceRef === 'string' ? value.sourceRef : ''
    let href = ''
    try {
      const url = new URL(source)
      if (url.protocol === 'https:' && !url.username && !url.password) href = url.href
    } catch {
      /* manual references remain plain text */
    }
    return [
      {
        title: typeof value.title === 'string' ? value.title : '',
        id: value.chunkId,
        excerpt: value.excerpt,
        source,
        href,
        version: value.version,
        review: evidenceReviewLabel(value.review),
      },
    ]
  })
}
export function plusUsage(value: unknown) {
  if (!value || typeof value !== 'object') return null
  const usage = value as Record<string, unknown>
  return {
    model: typeof usage.model === 'string' ? usage.model : 'Modelo desconhecido',
    tokens:
      typeof usage.tokens === 'number' && Number.isInteger(usage.tokens) && usage.tokens >= 0
        ? `${usage.tokens} tokens`
        : 'Uso desconhecido',
    cost:
      typeof usage.costUsd === 'string' && /^(0|[1-9]\d*)(\.\d{1,6})?$/.test(usage.costUsd)
        ? `Custo estimado USD ${usage.costUsd}`
        : 'Custo desconhecido',
    latency:
      typeof usage.latencyMs === 'number' && usage.latencyMs >= 0
        ? `${usage.latencyMs} ms`
        : 'Latência desconhecida',
  }
}
