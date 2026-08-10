// Helpers puros de video da task — extraidos de `useTasksPageContext.ts` (F-17 split).
//
// Sao funcoes puras (sem dependencia do closure reativo do contexto): normalizam metadata de
// video vinda da API/draft, calculam a assinatura de comparacao (autosave) e formatam o tamanho
// do arquivo. Ficam isoladas aqui para serem testaveis via Vitest e reduzir o tamanho do
// agregador. O contexto continua expondo `taskVideoSignature` re-bindando estas funcoes — o
// objeto/chaves de retorno do composable nao muda.

import type { TaskVideoItem } from '../types/tasks'
import { normalizeText } from './text'

export function normalizeTaskVideoItem(value: unknown): TaskVideoItem | null {
  if (!value || typeof value !== 'object') return null
  const raw = value as Record<string, unknown>
  const url = normalizeText(raw.url, 1000)
  const id = normalizeText(raw.id, 240) || url
  if (!id || !url) return null
  return {
    id,
    name: normalizeText(raw.name, 240) || id,
    url,
    size: Math.max(0, Number(raw.size || 0) || 0),
    contentType: normalizeText(raw.contentType, 120),
    checklistItemId: normalizeText(raw.checklistItemId, 120) || undefined,
    uploadedAt: normalizeText(raw.uploadedAt, 80),
  }
}

export function normalizeTaskVideoItems(value: unknown): TaskVideoItem[] {
  if (!Array.isArray(value)) return []
  const seen = new Set<string>()
  const normalized: TaskVideoItem[] = []
  value.forEach((item) => {
    const video = normalizeTaskVideoItem(item)
    if (!video || seen.has(video.id)) return
    seen.add(video.id)
    normalized.push(video)
  })
  return normalized
}

export function taskVideoSignature(value: unknown): string {
  return JSON.stringify(
    normalizeTaskVideoItems(value).map((video) => ({
      id: video.id,
      url: video.url,
      size: video.size,
      contentType: video.contentType,
      checklistItemId: video.checklistItemId || '',
    })),
  )
}

export function formatFileSize(size: number): string {
  if (!Number.isFinite(size) || size <= 0) return '0 KB'
  if (size < 1024 * 1024) return `${Math.max(1, Math.round(size / 1024))} KB`
  return `${(size / (1024 * 1024)).toFixed(size >= 10 * 1024 * 1024 ? 0 : 1)} MB`
}
