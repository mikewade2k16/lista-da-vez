import { computed, onMounted, reactive, ref, watch, type Ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  createAiKnowledgeBinding,
  createAiToolBinding,
  createKnowledgeBase,
  createKnowledgeDocument,
  disableAiKnowledgeBinding,
  disableAiToolBinding,
  fetchAgents,
  fetchAiKnowledgeBindings,
  fetchAiToolBindings,
  fetchKnowledgeBases,
  fetchKnowledgeDocuments,
  replaceKnowledgeDocumentChunks,
  updateAiKnowledgeBinding,
  updateAiToolBinding,
  updateKnowledgeBase,
  updateKnowledgeDocument,
} from '~/domain/omnichannel/config-api'
import {
  approveAiToolApproval,
  fetchAiToolApprovals,
  fetchAiToolRuns,
  rejectAiToolApproval,
} from '~/domain/omnichannel/ai-tools-api'
import type {
  OmniAgent,
  OmniAiKnowledgeBinding,
  OmniAiToolApproval,
  OmniAiToolBinding,
  OmniAiToolRun,
  OmniKnowledgeBase,
  OmniKnowledgeChunkInput,
  OmniKnowledgeDocument,
} from '~/domain/omnichannel/config-types'

export function useOmnichannelToolsKnowledge(
  canManage: Ref<boolean>,
  canAudit: Ref<boolean> = canManage,
) {
  const auth = useAuthStore()
  const ui = useUiStore()
  const runtimeConfig = useRuntimeConfig()
  const api = createApiRequest(runtimeConfig, () => auth.accessToken)

  const agents = ref<OmniAgent[]>([])
  const bases = ref<OmniKnowledgeBase[]>([])
  const toolBindings = ref<OmniAiToolBinding[]>([])
  const toolRuns = ref<OmniAiToolRun[]>([])
  const approvals = ref<OmniAiToolApproval[]>([])
  const knowledgeBindings = ref<OmniAiKnowledgeBinding[]>([])
  const documents = ref<OmniKnowledgeDocument[]>([])
  const selectedAgentId = ref('')
  const selectedBaseId = ref('')
  const loading = ref(true)
  const busy = ref(false)
  const newTool = reactive({ toolId: '', mode: 'read' as OmniAiToolBinding['mode'], operations: '', enabled: false })
  const newBase = reactive({ name: '', enabled: false })
  const newDocument = reactive({ sourceRef: '', title: '', checksum: '' })
  const editingDocumentId = ref('')
  const chunksDraft = ref('')

  const selectedAgent = computed(() => agents.value.find((agent) => agent.id === selectedAgentId.value) || null)
  const selectedBase = computed(() => bases.value.find((base) => base.id === selectedBaseId.value) || null)

  async function loadAgentData(): Promise<void> {
    if (!selectedAgentId.value) {
      toolBindings.value = []
      knowledgeBindings.value = []
      return
    }
    const [tools, knowledge, loadedApprovals, runs] = await Promise.all([
      fetchAiToolBindings(api, selectedAgentId.value),
      fetchAiKnowledgeBindings(api, selectedAgentId.value),
      canManage.value ? fetchAiToolApprovals(api, selectedAgentId.value, 30) : Promise.resolve([]),
      canAudit.value ? fetchAiToolRuns(api, selectedAgentId.value, { limit: 30 }) : Promise.resolve([]),
    ])
    toolBindings.value = tools
    knowledgeBindings.value = knowledge
    approvals.value = loadedApprovals
    toolRuns.value = runs
  }

  async function loadBaseData(): Promise<void> {
    documents.value = selectedBaseId.value ? await fetchKnowledgeDocuments(api, selectedBaseId.value) : []
  }

  async function loadRoot(): Promise<void> {
    loading.value = true
    try {
      const [loadedAgents, loadedBases] = await Promise.all([fetchAgents(api), fetchKnowledgeBases(api)])
      agents.value = loadedAgents
      bases.value = loadedBases
      if (!selectedAgentId.value) selectedAgentId.value = loadedAgents[0]?.id || ''
      if (!selectedBaseId.value) selectedBaseId.value = loadedBases[0]?.id || ''
      await Promise.all([loadAgentData(), loadBaseData()])
    } catch (error) {
      ui.error(getApiErrorMessage(error, 'Não foi possível carregar tools e conhecimento.'))
    } finally {
      loading.value = false
    }
  }

  function operations(): string[] {
    return newTool.operations.split(',').map((item) => item.trim()).filter(Boolean).slice(0, 32)
  }

  async function createTool(): Promise<void> {
    if (!canManage.value || busy.value || !newTool.toolId.trim() || !selectedAgentId.value) return
    busy.value = true
    try {
      await createAiToolBinding(api, selectedAgentId.value, {
        toolId: newTool.toolId.trim(), mode: newTool.mode, isEnabled: newTool.enabled,
        allowedOperations: operations(), inputSchema: {}, outputSchema: {}, timeoutMs: 5000, maxCallsPerDispatch: 4, config: {},
      })
      newTool.toolId = ''
      newTool.operations = ''
      ui.success('Binding salvo. A execução só ocorre se houver handler Go explicitamente registrado.')
      await loadAgentData()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível salvar o binding.'))
    } finally { busy.value = false }
  }

  async function toggleTool(binding: OmniAiToolBinding): Promise<void> {
    if (!canManage.value || busy.value) return
    busy.value = true
    try { await updateAiToolBinding(api, binding.agentId, binding.id, { isEnabled: !binding.isEnabled }); await loadAgentData()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível alterar a tool.'))
    } finally { busy.value = false }
  }

  async function disableTool(binding: OmniAiToolBinding): Promise<void> {
    if (!canManage.value || busy.value) return
    busy.value = true
    try { await disableAiToolBinding(api, binding.agentId, binding.id); ui.success('Tool desativada.'); await loadAgentData()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível desativar a tool.'))
    } finally { busy.value = false }
  }

  async function refreshToolRuns(): Promise<void> {
    if (!selectedAgentId.value || (!canManage.value && !canAudit.value)) return
    try {
      const [runs, loadedApprovals] = await Promise.all([
        canAudit.value ? fetchAiToolRuns(api, selectedAgentId.value, { limit: 30 }) : Promise.resolve([]),
        canManage.value ? fetchAiToolApprovals(api, selectedAgentId.value, 30) : Promise.resolve([]),
      ])
      toolRuns.value = runs
      approvals.value = loadedApprovals
    } catch (error) {
      ui.error(getApiErrorMessage(error, 'NÃ£o foi possÃ­vel carregar as evidÃªncias das tools.'))
    }
  }

  async function decideToolApproval(approval: OmniAiToolApproval, approved: boolean): Promise<void> {
    if (!canManage.value || busy.value || !selectedAgentId.value || approval.status !== 'pending') return
    busy.value = true
    try {
      if (approved) {
        await approveAiToolApproval(api, selectedAgentId.value, approval.id)
        ui.success('Proposta aprovada. O próximo retry assinado poderá executá-la no Go.')
      } else {
        await rejectAiToolApproval(api, selectedAgentId.value, approval.id)
        ui.success('Proposta negada e auditada.')
      }
      approvals.value = await fetchAiToolApprovals(api, selectedAgentId.value, 30)
    } catch (error) {
      ui.error(getApiErrorMessage(error, 'Não foi possível registrar a decisão.'))
    } finally {
      busy.value = false
    }
  }

  function formatToolJSON(value: Record<string, unknown>): string {
    try {
      return JSON.stringify(value, null, 2)
    } catch {
      return '{}'
    }
  }

  async function createBase(): Promise<void> {
    if (!canManage.value || busy.value || !newBase.name.trim()) return
    busy.value = true
    try {
      const base = await createKnowledgeBase(api, { name: newBase.name.trim(), isEnabled: newBase.enabled })
      newBase.name = ''
      bases.value = [...bases.value, base]
      selectedBaseId.value = base.id
      ui.success('Base criada desabilitada por padrão.')
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível criar a base.'))
    } finally { busy.value = false }
  }

  async function toggleBase(base: OmniKnowledgeBase): Promise<void> {
    if (!canManage.value || busy.value) return
    busy.value = true
    try { await updateKnowledgeBase(api, base.id, { isEnabled: !base.isEnabled }); await loadRoot()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível alterar a base.'))
    } finally { busy.value = false }
  }

  async function createDocument(): Promise<void> {
    if (!canManage.value || busy.value || !selectedBaseId.value || !newDocument.sourceRef.trim() || !newDocument.checksum.trim()) return
    busy.value = true
    try {
      await createKnowledgeDocument(api, selectedBaseId.value, { sourceRef: newDocument.sourceRef.trim(), title: newDocument.title.trim(), checksum: newDocument.checksum.trim(), metadata: {} })
      newDocument.sourceRef = ''; newDocument.title = ''; newDocument.checksum = ''
      ui.success('Documento criado como rascunho. Inclua chunks antes de publicar.')
      await loadBaseData()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível criar o documento.'))
    } finally { busy.value = false }
  }

  async function publishDocument(document: OmniKnowledgeDocument): Promise<void> {
    if (!canManage.value || busy.value || document.chunkCount < 1) return
    busy.value = true
    try { await updateKnowledgeDocument(api, document.knowledgeBaseId, document.id, { status: 'published' }); ui.success('Documento publicado para busca.'); await loadBaseData()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível publicar o documento.'))
    } finally { busy.value = false }
  }

  function startChunksEdit(document: OmniKnowledgeDocument): void {
    editingDocumentId.value = document.id
    chunksDraft.value = ''
  }

  function parseChunksDraft(): OmniKnowledgeChunkInput[] {
    return chunksDraft.value.split(/\n\s*---+\s*\n/g).map((bodyText, ordinal) => ({ ordinal, bodyText: bodyText.trim() })).filter((chunk) => chunk.bodyText.length > 0).slice(0, 500)
  }

  async function importKnowledgeFile(file: File | null): Promise<void> {
    if (!canManage.value || !file) return
    if (file.size > 2 * 1024 * 1024) {
      ui.error('O arquivo deve ter no máximo 2 MB.')
      return
    }
    try {
      const text = await file.text()
      const parts = text.split(/\n\s*\n/g).flatMap((part) => {
        const value = part.trim()
        return value ? value.match(/[\s\S]{1,16000}/g) || [] : []
      }).slice(0, 500)
      if (!parts.length) {
        ui.error('O arquivo não contém texto utilizável.')
        return
      }
      const checksum = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text))))
        .map((byte) => byte.toString(16).padStart(2, '0')).join('')
      const safeName = file.name.replace(/[^a-zA-Z0-9._-]/g, '_').slice(0, 120) || 'arquivo.txt'
      newDocument.sourceRef = `manual:${safeName}`
      newDocument.title = newDocument.title.trim() || file.name.slice(0, 500)
      newDocument.checksum = checksum
      chunksDraft.value = parts.join('\n---\n')
      ui.success(`${parts.length} chunks preparados. Crie o documento e salve os chunks.`)
    } catch (error) {
      ui.error(getApiErrorMessage(error, 'Não foi possível ler o arquivo.'))
    }
  }

  async function saveChunks(document: OmniKnowledgeDocument): Promise<void> {
    if (!canManage.value || busy.value || editingDocumentId.value !== document.id) return
    const chunks = parseChunksDraft()
    if (!chunks.length) { ui.error('Inclua ao menos um bloco de texto para criar os chunks.'); return }
    busy.value = true
    try {
      await replaceKnowledgeDocumentChunks(api, document.knowledgeBaseId, document.id, chunks)
      editingDocumentId.value = ''; chunksDraft.value = ''
      ui.success('Chunks substituídos. O documento continua em processamento até ser publicado.')
      await loadBaseData()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível salvar os chunks.'))
    } finally { busy.value = false }
  }

  async function createKnowledgeBinding(): Promise<void> {
    if (!canManage.value || busy.value || !selectedAgentId.value || !selectedBaseId.value) return
    busy.value = true
    try { await createAiKnowledgeBinding(api, selectedAgentId.value, { knowledgeBaseId: selectedBaseId.value, isEnabled: false, topK: 5, minScore: 0 }); ui.success('Binding de conhecimento criado desabilitado.'); await loadAgentData()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível vincular a base.'))
    } finally { busy.value = false }
  }

  async function toggleKnowledgeBinding(binding: OmniAiKnowledgeBinding): Promise<void> {
    if (!canManage.value || busy.value) return
    busy.value = true
    try { await updateAiKnowledgeBinding(api, binding.agentId, binding.id, { isEnabled: !binding.isEnabled }); await loadAgentData()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível alterar o vínculo.'))
    } finally { busy.value = false }
  }

  async function disableKnowledgeBinding(binding: OmniAiKnowledgeBinding): Promise<void> {
    if (!canManage.value || busy.value) return
    busy.value = true
    try { await disableAiKnowledgeBinding(api, binding.agentId, binding.id); await loadAgentData()
    } catch (error) { ui.error(getApiErrorMessage(error, 'Não foi possível desativar o vínculo.'))
    } finally { busy.value = false }
  }

  watch(selectedAgentId, () => void loadAgentData())
  watch(selectedBaseId, () => void loadBaseData())
  onMounted(() => void loadRoot())

  return {
    agents, bases, toolBindings, toolRuns, approvals, knowledgeBindings, documents, selectedAgentId, selectedBaseId,
    loading, busy, newTool, newBase, newDocument, editingDocumentId, chunksDraft,
    selectedAgent, selectedBase, parseChunksDraft, loadAgentData, loadBaseData, createTool,
    toggleTool, disableTool, refreshToolRuns, decideToolApproval, formatToolJSON, createBase, toggleBase, createDocument, publishDocument,
    startChunksEdit, importKnowledgeFile, saveChunks, createKnowledgeBinding, toggleKnowledgeBinding, disableKnowledgeBinding,
  }
}
