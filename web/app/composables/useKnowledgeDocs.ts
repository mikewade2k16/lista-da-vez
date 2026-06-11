import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

export interface KnowledgeDocView {
  id: string
  title: string
  body: string
  sortOrder: number
  enabled: boolean
}

export function useKnowledgeDocs() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const docs = ref<KnowledgeDocView[]>([])
  const loading = ref(false)
  const globalError = ref('')

  async function loadDocs() {
    loading.value = true
    globalError.value = ''
    try {
      docs.value = (await apiRequest('/v1/automation/knowledge-docs', {
        method: 'GET',
      })) as KnowledgeDocView[]
    } catch (e) {
      globalError.value = getApiErrorMessage(e, 'Falha ao carregar os documentos.')
    } finally {
      loading.value = false
    }
  }

  async function createDoc(title: string, body: string): Promise<KnowledgeDocView | null> {
    try {
      const doc = (await apiRequest('/v1/automation/knowledge-docs', {
        method: 'POST',
        body: { title, body },
      })) as KnowledgeDocView
      docs.value = [...docs.value, doc]
      return doc
    } catch (e) {
      globalError.value = getApiErrorMessage(e, 'Nao foi possivel criar o documento.')
      return null
    }
  }

  async function patchDoc(
    id: string,
    title: string,
    body: string,
    sortOrder: number,
    enabled: boolean,
  ): Promise<KnowledgeDocView | null> {
    try {
      const doc = (await apiRequest(`/v1/automation/knowledge-docs/${id}`, {
        method: 'PATCH',
        body: { title, body, sortOrder, enabled },
      })) as KnowledgeDocView
      docs.value = docs.value.map((d) => (d.id === doc.id ? doc : d))
      return doc
    } catch (e) {
      return null
    }
  }

  async function toggleEnabled(doc: KnowledgeDocView) {
    await patchDoc(doc.id, doc.title, doc.body, doc.sortOrder, !doc.enabled)
  }

  async function deleteDoc(id: string): Promise<boolean> {
    try {
      await apiRequest(`/v1/automation/knowledge-docs/${id}`, { method: 'DELETE' })
      docs.value = docs.value.filter((d) => d.id !== id)
      return true
    } catch (e) {
      globalError.value = getApiErrorMessage(e, 'Nao foi possivel apagar o documento.')
      return false
    }
  }

  return {
    docs,
    loading,
    globalError,
    loadDocs,
    createDoc,
    patchDoc,
    toggleEnabled,
    deleteDoc,
  }
}
