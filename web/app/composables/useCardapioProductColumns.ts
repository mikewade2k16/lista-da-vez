import { computed, onBeforeUnmount, ref, unref } from 'vue'
import type { MaybeRefOrGetter } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { formToPayload, productToForm } from '~/composables/useCardapioProductForm'
import { resolveMediaUrl } from '~/utils/media'
import type { Category, ProductListItem } from '~/domain/cardapio/types'
import type {
  OmniFilterDefinition,
  OmniSelectOption,
  OmniTableColumn,
} from '~/types/omni/collection'

// Campos editaveis inline na tabela de produtos. Cada um vira um PATCH
// full-replace (ver persistCell). `priceCents`/`compareAtPriceCents` chegam
// da OmniMoneyInput em REAIS e sao convertidos para centavos no commit.
export type CardapioProductFieldKey =
  | 'name'
  | 'categoryId'
  | 'priceCents'
  | 'compareAtPriceCents'
  | 'isAvailable'
  | 'isFeatured'

const EDITABLE_FIELDS = new Set<CardapioProductFieldKey>([
  'name',
  'categoryId',
  'priceCents',
  'compareAtPriceCents',
  'isAvailable',
  'isFeatured',
])

// Chaves de COLUNA editaveis inline (priceReais/compareAtPriceReais sao da UI;
// o modelo guarda em centavos). Usadas pelo overlay otimista.
const EDITABLE_COLUMN_KEYS = [
  'name',
  'categoryId',
  'priceReais',
  'compareAtPriceReais',
  'isAvailable',
  'isFeatured',
] as const

// Linha da tabela: projecao lean + preco em REAIS (OmniMoneyInput trabalha em
// reais, o modelo guarda centavos) + url de midia ja resolvida para a imagem.
export interface CardapioProductRow extends ProductListItem {
  priceReais: number
  compareAtPriceReais: number
  imageSrc: string
}

function centsToReais(cents: number): number {
  return Number((Math.max(0, Math.trunc(Number(cents) || 0)) / 100).toFixed(2))
}

function reaisToCents(reais: unknown): number {
  return Math.max(0, Math.round((Number(reais) || 0) * 100))
}

export interface CardapioProductFilters {
  categoryId?: unknown
  availability?: unknown
}

function resolveFilterState(source?: MaybeRefOrGetter<CardapioProductFilters>) {
  const raw = typeof source === 'function' ? source() : unref(source)
  return (raw ?? {}) as CardapioProductFilters
}

// Edicoes de texto/select/money confirmam apos uma pausa (nao a cada tecla);
// switches sao `immediate` e nao passam por aqui.
const COMMIT_DELAY_MS = 380

export interface CardapioColumnsCallbacks {
  onSuccess?: () => void
  onError?: (caught: unknown) => void
}

export function useCardapioProductColumns(
  filtersState?: MaybeRefOrGetter<CardapioProductFilters>,
  callbacks: CardapioColumnsCallbacks = {},
) {
  const store = useCardapioStore()
  const config = useRuntimeConfig()
  const apiBase = computed(() => String(config.public.apiBase || ''))

  // Overlay otimista: como os inputs da OmniDataTable sao controlados por
  // `row[column.key]` e o modelo so re-le do back depois do PATCH, guardamos o
  // valor digitado por (produto, coluna) ate o commit. Limpo apos persistir, para
  // o valor do banco voltar a mandar (fonte unica).
  const pendingEdits = ref<Record<string, unknown>>({})
  const pendingTimers = new Map<string, ReturnType<typeof setTimeout>>()

  function editKey(productId: string, columnKey: string) {
    return `${productId}:${columnKey}`
  }

  function setPendingEdit(key: string, value: unknown) {
    pendingEdits.value = { ...pendingEdits.value, [key]: value }
  }

  function clearPendingEdit(key: string) {
    if (!(key in pendingEdits.value)) return
    const next = { ...pendingEdits.value }
    Reflect.deleteProperty(next, key)
    pendingEdits.value = next
  }

  const categoryOptions = computed<OmniSelectOption[]>(() => [
    { label: 'Sem categoria', value: '' },
    ...store.categories
      .slice()
      .sort((a: Category, b: Category) => a.sortOrder - b.sortOrder)
      .map((category) => ({ label: category.name, value: category.id })),
  ])

  // Ordena por categoria (sortOrder da categoria) e depois por sortOrder do
  // produto, expondo a categoria como coluna/filtro (sem agrupamento na tabela,
  // que a OmniDataTable nao suporta de forma limpa).
  const tableRows = computed<CardapioProductRow[]>(() => {
    const categoryOrder = new Map<string, number>()
    store.categories.forEach((category, index) => {
      categoryOrder.set(category.id, category.sortOrder ?? index)
    })

    return store.products
      .slice()
      .sort((a, b) => {
        const orderA = categoryOrder.get(a.categoryId ?? '') ?? Number.MAX_SAFE_INTEGER
        const orderB = categoryOrder.get(b.categoryId ?? '') ?? Number.MAX_SAFE_INTEGER
        if (orderA !== orderB) return orderA - orderB
        return (a.sortOrder ?? 0) - (b.sortOrder ?? 0)
      })
      .map<CardapioProductRow>((product) => {
        const row: CardapioProductRow = {
          ...product,
          // OmniSelectInput compara por valor exato; categoria nula vira '' para
          // casar com a opcao "Sem categoria".
          categoryId: product.categoryId ?? '',
          priceReais: centsToReais(product.priceCents),
          // ATENCAO: a projecao lean (ProductLean no back) NAO traz
          // compareAtPriceCents — sempre 0 aqui. A edicao funciona (load-full),
          // mas o valor exibido nao reflete um preco riscado ja salvo ate o
          // produto ser carregado/editado. Backend gap (ver AGENT.md).
          compareAtPriceReais: 0,
          imageSrc: resolveMediaUrl(product.imageUrl, apiBase.value),
        }
        // Aplica o overlay otimista (edicao em voo) por cima da projecao do banco.
        const edits = pendingEdits.value
        for (const columnKey of EDITABLE_COLUMN_KEYS) {
          const key = editKey(product.id, columnKey)
          if (key in edits) {
            ;(row as unknown as Record<string, unknown>)[columnKey] = edits[key]
          }
        }
        return row
      })
  })

  // Filtragem client-side (a lista de produtos vem completa e lean; o
  // OmniCollectionFilters so atualiza o estado, nao filtra). Categoria casa por
  // id; disponibilidade por boolean. Vazio/'' = sem filtro.
  const filteredRows = computed<CardapioProductRow[]>(() => {
    const state = resolveFilterState(filtersState)
    const categoryFilter = String(state.categoryId ?? '').trim()
    const availabilityFilter = state.availability

    return tableRows.value.filter((row) => {
      if (categoryFilter && String(row.categoryId ?? '') !== categoryFilter) {
        return false
      }
      if (
        typeof availabilityFilter === 'boolean' &&
        Boolean(row.isAvailable) !== availabilityFilter
      ) {
        return false
      }
      return true
    })
  })

  const allTableColumns = computed<OmniTableColumn[]>(() => [
    {
      key: 'imageSrc',
      label: 'Imagem',
      type: 'image',
      editable: true,
      minWidth: 110,
      defaultOrder: 10,
    },
    {
      key: 'name',
      label: 'Nome',
      type: 'text',
      editable: true,
      locked: true,
      minWidth: 220,
      defaultOrder: 20,
    },
    {
      key: 'categoryId',
      label: 'Categoria',
      type: 'select',
      editable: true,
      options: categoryOptions.value,
      placeholder: 'Categoria',
      minWidth: 200,
      defaultOrder: 30,
    },
    {
      key: 'priceReais',
      label: 'Preço',
      type: 'money',
      editable: true,
      minWidth: 140,
      defaultOrder: 40,
    },
    {
      key: 'compareAtPriceReais',
      label: 'Preço comparativo',
      type: 'money',
      editable: true,
      minWidth: 160,
      defaultOrder: 50,
    },
    {
      key: 'isAvailable',
      label: 'Disponível',
      type: 'switch',
      editable: true,
      immediate: true,
      minWidth: 120,
      defaultOrder: 60,
    },
    {
      key: 'isFeatured',
      label: 'Destaque',
      type: 'switch',
      editable: true,
      immediate: true,
      minWidth: 110,
      defaultOrder: 70,
    },
    {
      key: 'actions',
      label: 'Ações',
      type: 'custom',
      minWidth: 120,
      align: 'center',
      defaultOrder: 1000,
    },
  ])

  const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
    {
      key: 'categoryId',
      label: 'Categoria',
      type: 'select',
      placeholder: 'Categoria',
      options: categoryOptions.value,
      accessor: (row) => String(row.categoryId ?? ''),
    },
    {
      key: 'availability',
      label: 'Disponibilidade',
      type: 'select',
      placeholder: 'Disponibilidade',
      options: [
        { label: 'Disponível', value: true },
        { label: 'Indisponível', value: false },
      ],
      accessor: (row) => Boolean(row.isAvailable),
    },
  ])

  // Mapeia a chave da COLUNA (priceReais/compareAtPriceReais sao da UI) para o
  // campo real do produto + o valor ja convertido para o modelo (centavos).
  function resolveField(
    key: string,
    value: unknown,
  ): { field: CardapioProductFieldKey; modelValue: unknown } | null {
    switch (key) {
      case 'name':
        return { field: 'name', modelValue: String(value ?? '') }
      case 'categoryId':
        return { field: 'categoryId', modelValue: String(value ?? '') }
      case 'priceReais':
        return { field: 'priceCents', modelValue: reaisToCents(value) }
      case 'compareAtPriceReais':
        return { field: 'compareAtPriceCents', modelValue: reaisToCents(value) }
      case 'isAvailable':
        return { field: 'isAvailable', modelValue: Boolean(value) }
      case 'isFeatured':
        return { field: 'isFeatured', modelValue: Boolean(value) }
      default:
        return null
    }
  }

  // Edicao inline com PATCH full-replace: a lista e lean e ProductInput nao e
  // parcial, entao busca o produto completo, mescla so o campo alterado, monta o
  // payload completo e faz PATCH. Espelha o toggleAvailable historico da secao.
  async function persistCell(productId: string, columnKey: string, value: unknown) {
    const resolved = resolveField(columnKey, value)
    if (!resolved || !EDITABLE_FIELDS.has(resolved.field)) return

    try {
      const full = await store.loadProduct(productId)
      const form = productToForm(full)

      switch (resolved.field) {
        case 'name':
          form.name = String(resolved.modelValue ?? '').trim()
          break
        case 'categoryId':
          form.categoryId = String(resolved.modelValue ?? '')
          break
        case 'priceCents':
          form.priceCents = Number(resolved.modelValue) || 0
          break
        case 'compareAtPriceCents':
          form.compareAtPriceCents = Number(resolved.modelValue) || 0
          break
        case 'isAvailable':
          form.isAvailable = Boolean(resolved.modelValue)
          break
        case 'isFeatured':
          form.isFeatured = Boolean(resolved.modelValue)
          break
      }

      await store.patchProduct(productId, formToPayload(form))
      callbacks.onSuccess?.()
    } catch (caught) {
      callbacks.onError?.(caught)
    } finally {
      // Limpa o overlay: o store ja re-leu do banco (reloadProducts no patch),
      // entao o valor autoritativo volta a mandar. Em erro, tambem limpa para
      // reverter o otimismo ao valor real.
      clearPendingEdit(editKey(productId, columnKey))
    }
  }

  // Commit da CELULA (nao a cada tecla): grava o valor no overlay otimista (a UI
  // reflete na hora) e agenda o PATCH. `immediate` (switches) persiste de uma vez;
  // texto/select/money debounce.
  function onCellInput(productId: string, columnKey: string, value: unknown, immediate?: boolean) {
    if (!productId) return
    const key = editKey(productId, columnKey)
    setPendingEdit(key, value)

    const existing = pendingTimers.get(key)
    if (existing) clearTimeout(existing)

    if (immediate) {
      void persistCell(productId, columnKey, value)
      return
    }

    pendingTimers.set(
      key,
      setTimeout(() => {
        pendingTimers.delete(key)
        void persistCell(productId, columnKey, value)
      }, COMMIT_DELAY_MS),
    )
  }

  // Sobe a imagem pela API de midia e grava a url via PATCH full-replace (mesma
  // mecanica de persistCell, mas a origem do valor e o upload).
  async function applyImageUpload(productId: string, file: File) {
    const url = await store.uploadMedia(productId, file)
    const full = await store.loadProduct(productId)
    const form = productToForm(full)
    form.imageUrl = url
    await store.patchProduct(productId, formToPayload(form))
  }

  onBeforeUnmount(() => {
    for (const timer of pendingTimers.values()) clearTimeout(timer)
    pendingTimers.clear()
  })

  return {
    categoryOptions,
    tableRows,
    filteredRows,
    allTableColumns,
    filterDefinitions,
    onCellInput,
    applyImageUpload,
  }
}
