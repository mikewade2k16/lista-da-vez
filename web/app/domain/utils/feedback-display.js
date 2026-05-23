export const feedbackKindOptions = [
  { value: '', label: 'Todos' },
  { value: 'suggestion', label: 'Sugestao' },
  { value: 'question', label: 'Duvida' },
  { value: 'problem', label: 'Problema' },
]

export const feedbackStatusOptions = [
  { value: '', label: 'Todos' },
  { value: 'open', label: 'Aberto' },
  { value: 'in_progress', label: 'Em analise' },
  { value: 'resolved', label: 'Resolvido' },
  { value: 'closed', label: 'Fechado' },
]

export const feedbackDetailStatusOptions = feedbackStatusOptions.filter((option) => option.value)

export function feedbackKindLabel(kind) {
  const labels = {
    problem: 'Problema',
    question: 'Duvida',
    suggestion: 'Sugestao',
  }

  return labels[String(kind || '').trim()] || kind || '-'
}

export function feedbackStatusLabel(status) {
  const labels = {
    closed: 'Fechado',
    in_progress: 'Em analise',
    open: 'Aberto',
    resolved: 'Resolvido',
  }

  return labels[String(status || '').trim()] || status || '-'
}

export function formatFeedbackDate(isoString) {
  try {
    return new Date(isoString).toLocaleDateString('pt-BR', {
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      month: '2-digit',
      year: 'numeric',
    })
  } catch {
    return isoString || ''
  }
}
