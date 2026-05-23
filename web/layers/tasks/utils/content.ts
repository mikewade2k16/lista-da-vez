const MODAL_EDITOR_ARTIFACT_TEXT_RE = /^(?:modal-editor(?:-retry)?-\d+\s*)+$/i

export function stripHtmlToText(value: unknown, max = 5000): string {
  return String(value ?? '')
    .replace(/<[^>]+>/g, ' ')
    .replace(/&nbsp;/gi, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

export function sanitizeTaskContentHtml(value: unknown, max = 50000): string {
  const html = String(value ?? '').trim().slice(0, max)
  const plainText = stripHtmlToText(html, max)
  if (plainText && MODAL_EDITOR_ARTIFACT_TEXT_RE.test(plainText)) {
    return ''
  }
  return html
}