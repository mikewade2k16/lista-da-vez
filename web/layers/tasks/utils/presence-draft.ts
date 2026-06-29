// Helpers puros de serializacao do draft de presence — extraidos de `useTasksPageContext.ts`
// (F-17 split).
//
// O draft efemero de presence (`presence.field_draft`) carrega valores estruturados (envolvidos,
// cliente, prazo) como string para trafegar no WS. Estas funcoes de encode/decode sao puras (so
// JSON + um prefixo magico para distinguir payload estruturado de string crua) e nao dependem do
// closure reativo. A serializacao/parse por campo (`serializePresenceDraftValue`/
// `parsePresenceDraftValue`) continua no contexto porque depende de `sanitizeInvolved` +
// `taskDraftResponsibleValue` (estado reativo), mas reusa estes helpers como base.

const STRUCTURED_PRESENCE_DRAFT_PREFIX = '__tasks_presence_json__:'

export function encodeStructuredPresenceDraft(value: unknown): string {
  try {
    return `${STRUCTURED_PRESENCE_DRAFT_PREFIX}${JSON.stringify(value ?? null)}`
  } catch {
    return `${STRUCTURED_PRESENCE_DRAFT_PREFIX}null`
  }
}

export function decodeStructuredPresenceDraft<T>(value: unknown): T | null {
  if (typeof value !== 'string' || !value.startsWith(STRUCTURED_PRESENCE_DRAFT_PREFIX)) return null
  try {
    return JSON.parse(value.slice(STRUCTURED_PRESENCE_DRAFT_PREFIX.length)) as T
  } catch {
    return null
  }
}
