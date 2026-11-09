import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { createApiRequest } from '~/utils/api-client'
import type {
  PublicAttempt,
  PublicCatalogStatus,
  PublicChunk,
  PublicCollection,
  PublicReview,
  PublicReviewInput,
} from '~/types/public-knowledge'

// Curadoria do acervo público do chat (somente platform_admin). Sem cache: a tela
// relê o catálogo depois de cada ação (fonte única no banco).
export const usePublicKnowledgeStore = defineStore('public-knowledge', () => {
  const auth = useAuthStore()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const base = '/v1/knowledge/public'
  const call = async <T>(path: string, options: Record<string, unknown> = {}) => {
    const response = (await api(`${base}${path}`, {
      ...options,
      dedupe: false,
      skipLoadingIndicator: true,
    })) as { data: T }
    return response.data
  }
  const id = (value: string) => encodeURIComponent(value)
  return {
    catalog: (signal?: AbortSignal) => call<PublicCatalogStatus[]>('/catalog', { signal }),
    collect: (key: string) =>
      call<PublicAttempt>(`/catalog/${id(key)}/collect`, {
        method: 'POST',
        timeout: 180000,
        retry: 0,
      }),
    chunks: (collection: string, page: number) =>
      call<PublicChunk[]>(`/collections/${id(collection)}/chunks?page=${page}`),
    excludeChunk: (collection: string, chunk: string) =>
      call<{ excluded: boolean }>(`/collections/${id(collection)}/chunks/${id(chunk)}`, {
        method: 'DELETE',
      }),
    reviews: (collection: string) => call<PublicReview[]>(`/collections/${id(collection)}/reviews`),
    review: (collection: string, input: PublicReviewInput) =>
      call<PublicReview>(`/collections/${id(collection)}/reviews`, { method: 'POST', body: input }),
    publish: (collection: string) =>
      call<PublicCollection>(`/collections/${id(collection)}/publish`, { method: 'POST' }),
    discard: (collection: string) =>
      call<{ discarded: boolean }>(`/collections/${id(collection)}/discard`, { method: 'POST' }),
    async exportText(collection: string): Promise<Blob> {
      return (await api(`${base}/collections/${id(collection)}/export`, {
        responseType: 'blob',
        dedupe: false,
        skipLoadingIndicator: true,
      })) as Blob
    },
  }
})
