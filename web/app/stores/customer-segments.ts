import { defineStore } from 'pinia'
import {
  createSegmentExport,
  createSegment,
  fetchSegment,
  fetchSegmentFieldCatalog,
  fetchSegmentMaterializations,
  fetchSegments,
  runSegmentVersionAction,
  updateSegmentDraft,
} from '~/domain/customer-data/segment-api'
import type {
  CustomerSegmentListItem,
  CustomerSegmentView,
  SegmentEvaluationRun,
  SegmentExportView,
  SegmentFieldCatalog,
  SegmentFilterAst,
  SegmentMaterializationView,
} from '~/domain/customer-data/segment-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { createApiRequest } from '~/utils/api-client'
import { useAuthStore } from './auth'

type LoadStatus = 'idle' | 'loading' | 'ready' | 'error'

function requestKey(): string {
  return globalThis.crypto?.randomUUID?.() ?? `panel-${Date.now()}`
}

export const useCustomerSegmentsStore = defineStore('customer-segments', {
  state: () => ({
    ownerAccountId: '',
    clientAccountId: '',
    canFetch: false,
    generation: 0,
    controllers: new Set<AbortController>(),
    catalog: null as SegmentFieldCatalog | null,
    segments: [] as CustomerSegmentListItem[],
    nextCursor: '',
    selected: null as CustomerSegmentView | null,
    localAst: null as SegmentFilterAst | null,
    baselineAst: '',
    evaluation: null as SegmentEvaluationRun | null,
    lastExport: null as SegmentExportView | null,
    materializations: [] as SegmentMaterializationView[],
    status: 'idle' as LoadStatus,
    actionStatus: 'idle' as LoadStatus,
    error: null as CustomerApiErrorState | null,
  }),

  getters: {
    scopeKey: (state): string => `${state.ownerAccountId}:${state.clientAccountId}`,
    dirty: (state): boolean =>
      Boolean(state.localAst) && JSON.stringify(state.localAst) !== state.baselineAst,
  },

  actions: {
    abortAll(): void {
      this.controllers.forEach((controller) => controller.abort())
      this.controllers.clear()
    },

    clearProtectedState(): void {
      this.catalog = null
      this.segments = []
      this.nextCursor = ''
      this.selected = null
      this.localAst = null
      this.baselineAst = ''
      this.evaluation = null
      this.lastExport = null
      this.materializations = []
      this.status = 'idle'
      this.actionStatus = 'idle'
      this.error = null
    },

    setScope(ownerAccountId: string, clientAccountId: string, canFetch: boolean): void {
      const owner = String(ownerAccountId || '').trim()
      const client = String(clientAccountId || '').trim()
      if (
        owner === this.ownerAccountId &&
        client === this.clientAccountId &&
        canFetch === this.canFetch
      ) {
        return
      }
      this.abortAll()
      this.generation += 1
      this.ownerAccountId = owner
      this.clientAccountId = client
      this.canFetch = canFetch
      this.clearProtectedState()
    },

    beginRequest(): { controller: AbortController; generation: number } {
      const controller = new AbortController()
      this.controllers.add(controller)
      return { controller, generation: this.generation }
    },

    requestIsCurrent(controller: AbortController, generation: number): boolean {
      return !controller.signal.aborted && generation === this.generation
    },

    finishRequest(controller: AbortController): void {
      this.controllers.delete(controller)
    },

    async loadWorkspace(status = '', append = false): Promise<void> {
      if (!this.canFetch || !this.clientAccountId) return
      const auth = useAuthStore()
      const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
      const request = this.beginRequest()
      this.status = 'loading'
      this.error = null
      try {
        const [catalog, page] = await Promise.all([
          this.catalog
            ? Promise.resolve(this.catalog)
            : fetchSegmentFieldCatalog(api, this.clientAccountId, request.controller.signal),
          fetchSegments(
            api,
            {
              clientAccountId: this.clientAccountId,
              status,
              cursor: append ? this.nextCursor : '',
            },
            request.controller.signal,
          ),
        ])
        if (!this.requestIsCurrent(request.controller, request.generation)) return
        this.catalog = catalog
        this.segments = append ? [...this.segments, ...page.items] : page.items
        this.nextCursor = page.nextCursor
        this.status = 'ready'
      } catch (cause) {
        if (!this.requestIsCurrent(request.controller, request.generation)) return
        this.error = classifyCustomerApiError(cause, 'Segmentos indisponiveis.')
        this.status = 'error'
      } finally {
        this.finishRequest(request.controller)
      }
    },

    async selectSegment(segmentId: string): Promise<boolean> {
      if (!this.canFetch || !this.clientAccountId || this.dirty) return false
      const auth = useAuthStore()
      const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
      const request = this.beginRequest()
      this.actionStatus = 'loading'
      this.error = null
      try {
        const [selected, materializations] = await Promise.all([
          fetchSegment(api, segmentId, this.clientAccountId, request.controller.signal),
          fetchSegmentMaterializations(
            api,
            segmentId,
            this.clientAccountId,
            request.controller.signal,
          ),
        ])
        if (!this.requestIsCurrent(request.controller, request.generation)) return false
        this.selected = selected
        this.materializations = materializations
        this.localAst = selected.draft?.filterAst ? structuredClone(selected.draft.filterAst) : null
        this.baselineAst = JSON.stringify(this.localAst)
        this.evaluation = null
        this.lastExport = null
        this.actionStatus = 'ready'
        return true
      } catch (cause) {
        if (!this.requestIsCurrent(request.controller, request.generation)) return false
        this.error = classifyCustomerApiError(cause, 'Segmento indisponivel.')
        this.actionStatus = 'error'
        return false
      } finally {
        this.finishRequest(request.controller)
      }
    },

    setLocalAst(ast: SegmentFilterAst): void {
      this.localAst = structuredClone(ast)
    },

    discardDraft(): void {
      this.localAst = this.selected?.draft?.filterAst
        ? structuredClone(this.selected.draft.filterAst)
        : null
      this.baselineAst = JSON.stringify(this.localAst)
    },

    async createNew(name: string, segmentKey: string): Promise<boolean> {
      if (!this.canFetch || !this.catalog || !name.trim() || !segmentKey.trim()) return false
      const auth = useAuthStore()
      const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
      this.actionStatus = 'loading'
      this.error = null
      const emptyAst: SegmentFilterAst = {
        schemaVersion: this.catalog.schemaVersion,
        root: { kind: 'group', nodeId: requestKey(), combinator: 'and', children: [] },
      }
      try {
        const created = await createSegment(api, {
          clientAccountId: this.clientAccountId,
          segmentKey: segmentKey.trim(),
          name: name.trim(),
          description: '',
          draft: {
            filterSchemaVersion: this.catalog.schemaVersion,
            fieldCatalogVersion: this.catalog.version,
            filterAst: emptyAst,
            evaluationPolicy: {},
          },
          idempotencyKey: requestKey(),
        })
        this.selected = created
        this.localAst = structuredClone(created.draft?.filterAst ?? emptyAst)
        this.baselineAst = JSON.stringify(this.localAst)
        await this.loadWorkspace()
        this.actionStatus = 'ready'
        return true
      } catch (cause) {
        this.error = classifyCustomerApiError(cause, 'Nao foi possivel criar o segmento.')
        this.actionStatus = 'error'
        return false
      }
    },

    async saveDraft(): Promise<boolean> {
      const draft = this.selected?.draft
      const segmentId = this.selected?.segment.id
      if (!draft || !segmentId || !this.localAst || !this.catalog || !this.dirty) {
        return false
      }
      const auth = useAuthStore()
      const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
      this.actionStatus = 'loading'
      this.error = null
      try {
        await updateSegmentDraft(api, draft.id, {
          filterAst: this.localAst,
          fieldCatalogVersion: this.catalog.version,
          expectedRevision: draft.revision,
        })
        return await this.selectAfterMutation(segmentId)
      } catch (cause) {
        this.error = classifyCustomerApiError(cause, 'Nao foi possivel salvar o draft.')
        this.actionStatus = 'error'
        return false
      }
    },

    async versionAction(action: 'validate' | 'preview' | 'publish'): Promise<boolean> {
      const draft = this.selected?.draft
      const segmentId = this.selected?.segment.id
      if (!draft || !segmentId || this.dirty) return false
      const auth = useAuthStore()
      const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
      this.actionStatus = 'loading'
      this.error = null
      try {
        const result = await runSegmentVersionAction(api, draft.id, action, {
          expectedRevision: draft.revision,
          idempotencyKey: requestKey(),
        })
        if ('mode' in result) this.evaluation = result
        if (action !== 'preview') await this.selectAfterMutation(segmentId)
        this.actionStatus = 'ready'
        return true
      } catch (cause) {
        this.error = classifyCustomerApiError(cause, `Falha ao executar ${action}.`)
        this.actionStatus = 'error'
        return false
      }
    },

    async requestExport(input: {
      materializationId: string
      purposeKey: string
      channelKey: string
      formatKey: string
      fieldSetKey: string
      reason?: string
    }): Promise<boolean> {
      if (!this.canFetch || !this.selected) return false
      const auth = useAuthStore()
      const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
      this.actionStatus = 'loading'
      this.error = null
      try {
        this.lastExport = await createSegmentExport(api, {
          clientAccountId: this.clientAccountId,
          ...input,
          idempotencyKey: requestKey(),
        })
        this.actionStatus = 'ready'
        return true
      } catch (cause) {
        this.error = classifyCustomerApiError(cause, 'Exportacao indisponivel.')
        this.actionStatus = 'error'
        return false
      }
    },

    async selectAfterMutation(segmentId: string): Promise<boolean> {
      this.baselineAst = ''
      this.localAst = null
      return this.selectSegment(segmentId)
    },
  },
})
