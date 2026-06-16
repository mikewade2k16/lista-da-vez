import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import type {
  BioCreatePayload,
  BioData,
  BioDefaults,
  BioDetail,
  BioListFilters,
  BioMediaKind,
  BioMediaUploadResult,
  BioPatchPayload,
  BioSlide,
  BioSlideSource,
  BioSlideSourceFacets,
  BioSlideSourceOption,
  BioSummary,
} from '~/domain/bio/types'

// Store do modulo bio (painel Site/Bio). Contrato congelado em
// docs/bio/PLANO_MODULO_BIO.md §4 — os componentes em web/app/components/bio/*
// dependem destes nomes (estados/computeds/actions). X-Account-Id e injetado
// automaticamente pelo createApiRequest (account ativa); a rota multi-tenant
// /v1/bio gateia por modulo no backend.
//
// accountId de query e FILTRO dentro do permitido: nao-admin so ve a propria
// account; pedir outra retorna 404. O front gateia o filtro de cliente por
// papel (so platform_admin) — espelhando o back.

// O PATCH passa a aceitar accountId (mover de account, so platform_admin). O
// type BioPatchPayload (domain/bio/types.ts) e do subagente C; estendemos aqui
// localmente para nao acoplar a interface compartilhada das secoes.
type BioPatchInput = BioPatchPayload & { accountId?: string }

function isPlatformAdmin(role: string): boolean {
  return String(role || '').trim() === 'platform_admin'
}

function buildListQuery(filters: BioListFilters): string {
  const params = new URLSearchParams()
  const accountId = String(filters.accountId || '').trim()
  const status = String(filters.status || '').trim()
  const q = String(filters.q || '').trim()

  if (accountId) {
    params.set('accountId', accountId)
  }
  if (status) {
    params.set('status', status)
  }
  if (q) {
    params.set('q', q)
  }

  const query = params.toString()
  return query ? `?${query}` : ''
}

export const useBioStore = defineStore('bio', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const bios = ref<BioSummary[]>([])
  const listPending = ref(false)
  const listError = ref('')

  const activeBio = ref<BioDetail | null>(null)
  const detailPending = ref(false)
  const detailError = ref('')

  const defaults = ref<BioDefaults | null>(null)
  const defaultsPending = ref(false)
  const defaultsError = ref('')

  // Fonte de produtos (B7): fontes disponiveis para a account + facets do
  // site.products. X-Account-Id e injetado pelo createApiRequest; o backend
  // escopa por account (nao-admin nunca ve dado de outra account).
  const sources = ref<BioSlideSourceOption[]>([])
  const sourcesPending = ref(false)
  const facets = ref<BioSlideSourceFacets>({ categories: [], campaigns: [], tipos: [] })
  const facetsPending = ref(false)
  const facetsError = ref('')

  const isAdmin = computed(() => isPlatformAdmin(auth.role))

  function resetActive() {
    activeBio.value = null
    detailError.value = ''
  }

  async function loadBios(filters: BioListFilters = {}) {
    listPending.value = true
    listError.value = ''
    try {
      const response = (await apiRequest(`/v1/bio/bios${buildListQuery(filters)}`, {
        method: 'GET',
      })) as { bios?: BioSummary[] } | BioSummary[]
      bios.value = Array.isArray(response) ? response : (response?.bios ?? [])
    } catch (caught) {
      listError.value = getApiErrorMessage(caught, 'Nao foi possivel carregar as bios.')
      bios.value = []
    } finally {
      listPending.value = false
    }
  }

  async function loadBio(id: string): Promise<BioDetail | null> {
    const bioId = String(id || '').trim()
    if (!bioId) {
      return null
    }

    detailPending.value = true
    detailError.value = ''
    try {
      const response = (await apiRequest(`/v1/bio/bios/${encodeURIComponent(bioId)}`, {
        method: 'GET',
      })) as BioDetail
      activeBio.value = response
      return response
    } catch (caught) {
      detailError.value = getApiErrorMessage(caught, 'Nao foi possivel carregar a bio.')
      activeBio.value = null
      return null
    } finally {
      detailPending.value = false
    }
  }

  async function createBio(payload: BioCreatePayload) {
    const slug = String(payload.slug || '').trim()
    const name = String(payload.name || '').trim()

    if (!name) {
      return { ok: false as const, message: 'Preencha o nome da bio.' }
    }

    const body: BioCreatePayload = { slug, name }
    // accountId e OPCIONAL: vazio (admin) = account do contexto (a agencia).
    // Nao-admin nunca envia accountId (usa o do contexto via X-Account-Id).
    if (isAdmin.value && payload.accountId) {
      body.accountId = String(payload.accountId).trim()
    }

    try {
      const response = (await apiRequest('/v1/bio/bios', {
        method: 'POST',
        body,
      })) as BioDetail
      return { ok: true as const, bio: response }
    } catch (caught) {
      return {
        ok: false as const,
        message: getApiErrorMessage(caught, 'Nao foi possivel criar a bio.'),
      }
    }
  }

  // Duplica uma bio: o backend copia o data_draft da origem numa bio nova
  // (status draft, slug unico, name "Copia de ..."). Retorna a BioView nova.
  async function duplicateBio(id: string) {
    const bioId = String(id || '').trim()
    if (!bioId) {
      return { ok: false as const, message: 'Bio invalida.' }
    }

    try {
      const response = (await apiRequest(`/v1/bio/bios/${encodeURIComponent(bioId)}/duplicate`, {
        method: 'POST',
      })) as { bio?: BioDetail } | BioDetail
      const bio = (response as { bio?: BioDetail }).bio ?? (response as BioDetail)
      return { ok: true as const, bio }
    } catch (caught) {
      return {
        ok: false as const,
        message: getApiErrorMessage(caught, 'Nao foi possivel duplicar a bio.'),
      }
    }
  }

  // Move a bio para outra account (so platform_admin). O backend ignora
  // accountId de quem nao e admin; o front so chama quando isAdmin.
  async function moveBioAccount(id: string, accountId: string) {
    const bioId = String(id || '').trim()
    const target = String(accountId || '').trim()
    if (!bioId || !target) {
      return { ok: false as const, message: 'Bio ou cliente invalido.' }
    }

    return patchBio(bioId, { accountId: target })
  }

  async function patchBio(id: string, payload: BioPatchInput) {
    const bioId = String(id || '').trim()
    if (!bioId) {
      return { ok: false as const, message: 'Bio invalida.' }
    }

    try {
      const response = (await apiRequest(`/v1/bio/bios/${encodeURIComponent(bioId)}`, {
        method: 'PATCH',
        body: payload,
      })) as BioDetail
      if (activeBio.value?.id === bioId) {
        activeBio.value = response
      }
      return { ok: true as const, bio: response }
    } catch (caught) {
      return {
        ok: false as const,
        message: getApiErrorMessage(caught, 'Nao foi possivel salvar a bio.'),
      }
    }
  }

  async function publishBio(id: string) {
    return mutateStatus(id, 'publish', 'Nao foi possivel publicar a bio.')
  }

  async function unpublishBio(id: string) {
    return mutateStatus(id, 'unpublish', 'Nao foi possivel despublicar a bio.')
  }

  async function mutateStatus(id: string, action: 'publish' | 'unpublish', fallback: string) {
    const bioId = String(id || '').trim()
    if (!bioId) {
      return { ok: false as const, message: 'Bio invalida.' }
    }

    try {
      const response = (await apiRequest(`/v1/bio/bios/${encodeURIComponent(bioId)}/${action}`, {
        method: 'POST',
      })) as BioDetail
      if (activeBio.value?.id === bioId) {
        activeBio.value = response
      }
      return { ok: true as const, bio: response }
    } catch (caught) {
      return { ok: false as const, message: getApiErrorMessage(caught, fallback) }
    }
  }

  async function deleteBio(id: string) {
    const bioId = String(id || '').trim()
    if (!bioId) {
      return { ok: false as const, message: 'Bio invalida.' }
    }

    try {
      await apiRequest(`/v1/bio/bios/${encodeURIComponent(bioId)}`, { method: 'DELETE' })
      bios.value = bios.value.filter((bio) => bio.id !== bioId)
      if (activeBio.value?.id === bioId) {
        resetActive()
      }
      return { ok: true as const }
    } catch (caught) {
      return {
        ok: false as const,
        message: getApiErrorMessage(caught, 'Nao foi possivel excluir a bio.'),
      }
    }
  }

  async function previewBio(id: string): Promise<BioData | null> {
    const bioId = String(id || '').trim()
    if (!bioId) {
      return null
    }

    try {
      return (await apiRequest(`/v1/bio/bios/${encodeURIComponent(bioId)}/preview`, {
        method: 'GET',
      })) as BioData
    } catch {
      return null
    }
  }

  async function uploadMedia(
    id: string,
    kind: BioMediaKind,
    file: File,
  ): Promise<BioMediaUploadResult | null> {
    const bioId = String(id || '').trim()
    if (!bioId || !file) {
      return null
    }

    const form = new FormData()
    form.append('kind', kind)
    form.append('file', file)

    return (await apiRequest(`/v1/bio/bios/${encodeURIComponent(bioId)}/media`, {
      method: 'POST',
      body: form,
    })) as BioMediaUploadResult
  }

  // Fontes disponiveis para a account (GET /v1/bio/sources). MVP devolve
  // site_products. Usado pelo seletor de fonte da secao Slides.
  async function loadSources(): Promise<BioSlideSourceOption[]> {
    sourcesPending.value = true
    try {
      const response = (await apiRequest('/v1/bio/sources', { method: 'GET' })) as
        | { sources?: BioSlideSourceOption[] }
        | BioSlideSourceOption[]
      sources.value = Array.isArray(response) ? response : (response?.sources ?? [])
      return sources.value
    } catch {
      sources.value = []
      return []
    } finally {
      sourcesPending.value = false
    }
  }

  // Facets distintos de site.products da account (GET .../facets). Popula os
  // selects de categoria/campanha/tipo do modo "Produtos do site".
  async function loadSiteProductFacets(): Promise<BioSlideSourceFacets> {
    facetsPending.value = true
    facetsError.value = ''
    const empty: BioSlideSourceFacets = { categories: [], campaigns: [], tipos: [] }
    try {
      const response = (await apiRequest('/v1/bio/sources/site_products/facets', {
        method: 'GET',
      })) as Partial<BioSlideSourceFacets> | null
      facets.value = {
        categories: response?.categories ?? [],
        campaigns: response?.campaigns ?? [],
        tipos: response?.tipos ?? [],
      }
      return facets.value
    } catch (caught) {
      facetsError.value = getApiErrorMessage(
        caught,
        'Nao foi possivel carregar os filtros de produtos.',
      )
      facets.value = empty
      return empty
    } finally {
      facetsPending.value = false
    }
  }

  // Resolve os slides de uma fonte para a PREVIA do editor (ver os produtos da
  // fonte ANTES de publicar). Espelha o que o publico faz em resolveSlideSource,
  // mas sem depender de bio publicada — os filtros vem direto do source do draft.
  // Retorna no shape BioSlide (src/title + whatsapp=href p/ o link do slide).
  async function resolvePreviewSlides(
    source: BioSlideSource | undefined,
    whatsapp: string,
  ): Promise<BioSlide[]> {
    const type = String(source?.type || '').trim()
    if (!type || type === 'manual') {
      return []
    }
    const params = new URLSearchParams()
    if (source?.category) params.set('category', source.category)
    if (source?.campaigns?.length) params.set('campaigns', source.campaigns.join(','))
    if (source?.tipo) params.set('tipo', source.tipo)
    if (typeof source?.limit === 'number' && source.limit > 0)
      params.set('limit', String(source.limit))
    if (source?.link) params.set('link', source.link)
    if (whatsapp) params.set('whatsapp', whatsapp)
    try {
      const resp = (await apiRequest(
        `/v1/bio/sources/${encodeURIComponent(type)}/resolve?${params.toString()}`,
        { method: 'GET' },
      )) as {
        slides?: Array<{
          src: string
          title?: string
          desc?: string
          price?: string
          href?: string
        }>
      } | null
      // Mesmo shape do resolvedToAny do backend (src/title/desc/price/href) — a
      // previa bate exatamente com o publico (o SlideTopKeen usa href || whatsapp
      // e o Lightbox exibe title/desc/price).
      return (resp?.slides ?? []).map((s) => ({
        src: s.src,
        title: s.title,
        desc: s.desc,
        price: s.price,
        href: s.href,
      }))
    } catch {
      return []
    }
  }

  async function loadDefaults() {
    if (!isAdmin.value) {
      return null
    }

    defaultsPending.value = true
    defaultsError.value = ''
    try {
      const response = (await apiRequest('/v1/bio/defaults', { method: 'GET' })) as BioDefaults
      defaults.value = response
      return response
    } catch (caught) {
      defaultsError.value = getApiErrorMessage(caught, 'Nao foi possivel carregar os defaults.')
      return null
    } finally {
      defaultsPending.value = false
    }
  }

  async function saveDefaults(data: BioData) {
    if (!isAdmin.value) {
      return { ok: false as const, message: 'Somente o admin da plataforma edita os defaults.' }
    }

    defaultsPending.value = true
    defaultsError.value = ''
    try {
      const response = (await apiRequest('/v1/bio/defaults', {
        method: 'PUT',
        body: { data },
      })) as BioDefaults
      defaults.value = response
      return { ok: true as const, defaults: response }
    } catch (caught) {
      const message = getApiErrorMessage(caught, 'Nao foi possivel salvar os defaults.')
      defaultsError.value = message
      return { ok: false as const, message }
    } finally {
      defaultsPending.value = false
    }
  }

  return {
    bios,
    listPending,
    listError,
    activeBio,
    detailPending,
    detailError,
    defaults,
    defaultsPending,
    defaultsError,
    sources,
    sourcesPending,
    facets,
    facetsPending,
    facetsError,
    isAdmin,
    resetActive,
    loadBios,
    loadBio,
    createBio,
    duplicateBio,
    moveBioAccount,
    patchBio,
    publishBio,
    unpublishBio,
    deleteBio,
    previewBio,
    uploadMedia,
    loadSources,
    loadSiteProductFacets,
    resolvePreviewSlides,
    loadDefaults,
    saveDefaults,
  }
})
