import type { ComputedRef } from 'vue'
import type {
  OmniFilterDefinition,
  OmniSelectOption,
  OmniTableColumn,
} from '~/types/omni/collection'

// Definicao das colunas da tabela de produtos do site + dos filtros. Extraido do
// workspace para mante-lo enxuto (< 500 linhas). As colunas de Categorias/
// Campanhas e os filtros recebem as options (facets da account) por parametro
// reativo, entao mostram a lista COMPLETA mesmo no modo paginado.
function arrayIncludesTag(value: unknown, target: unknown): boolean {
  const tag = String(target ?? '').trim()
  if (!tag || !Array.isArray(value)) return false
  return value.some((item) => String(item ?? '').trim() === tag)
}

export function useSiteProductColumns(
  categoryOptions: ComputedRef<OmniSelectOption[]>,
  campaignOptions: ComputedRef<OmniSelectOption[]>,
) {
  const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
    {
      key: 'query',
      label: 'Buscar',
      type: 'text',
      placeholder: 'Nome, codigo, descricao...',
      mode: 'all',
    },
    {
      key: 'statusFilter',
      label: 'Status',
      type: 'select',
      placeholder: 'Status',
      options: [
        { label: 'Ativo', value: 'active' },
        { label: 'Inativo', value: 'inactive' },
      ],
      accessor: (row) => row.status,
    },
    {
      key: 'categoryFilter',
      label: 'Categoria',
      type: 'select',
      placeholder: 'Categoria',
      options: categoryOptions.value,
      customPredicate: (row, value) => arrayIncludesTag(row.categories, value),
    },
    {
      key: 'campaignFilter',
      label: 'Campanha',
      type: 'select',
      placeholder: 'Campanha',
      options: campaignOptions.value,
      customPredicate: (row, value) => arrayIncludesTag(row.campaigns, value),
    },
  ])

  const allTableColumns = computed<OmniTableColumn[]>(() => [
    {
      key: 'image',
      label: 'Imagem',
      type: 'image',
      editable: true,
      align: 'center',
      minWidth: 110,
      defaultOrder: 5,
    },
    {
      key: 'name',
      label: 'Nome',
      type: 'text',
      editable: true,
      minWidth: 220,
      focusOnCreate: true,
      locked: true,
      defaultOrder: 10,
    },
    { key: 'code', label: 'Codigo', type: 'text', editable: true, minWidth: 140, defaultOrder: 20 },
    {
      key: 'status',
      label: 'Visivel no site',
      type: 'switch',
      editable: true,
      immediate: true,
      align: 'center',
      minWidth: 130,
      defaultOrder: 30,
      switchOnValue: 'active',
      switchOffValue: 'inactive',
    },
    {
      key: 'hasStock',
      label: 'Tem estoque',
      type: 'switch',
      editable: true,
      immediate: true,
      align: 'center',
      minWidth: 120,
      defaultOrder: 32,
      switchOnValue: true,
      switchOffValue: false,
    },
    {
      key: 'categories',
      label: 'Categorias',
      type: 'multiselect',
      editable: true,
      creatable: true,
      minWidth: 200,
      defaultOrder: 34,
      options: categoryOptions.value,
    },
    {
      key: 'campaigns',
      label: 'Campanhas',
      type: 'multiselect',
      editable: true,
      creatable: true,
      minWidth: 200,
      defaultOrder: 38,
      options: campaignOptions.value,
    },
    {
      key: 'price',
      label: 'Preco',
      type: 'money',
      editable: true,
      minWidth: 140,
      defaultOrder: 40,
    },
    {
      key: 'fator',
      label: 'Fator',
      type: 'number',
      editable: true,
      minWidth: 100,
      defaultOrder: 50,
    },
    {
      key: 'stock',
      label: 'Estoque',
      type: 'number',
      editable: true,
      minWidth: 110,
      defaultOrder: 60,
    },
    { key: 'tipo', label: 'Tipo', type: 'text', editable: true, minWidth: 130, defaultOrder: 70 },
    {
      key: 'sourceLabel',
      label: 'Fonte',
      type: 'text',
      editable: false,
      minWidth: 140,
      defaultOrder: 80,
    },
    {
      // Indicador de cruzamento com o ERP (badge verde quando vinculado). Slot
      // custom no workspace: cell-erpSynced.
      key: 'erpSynced',
      label: 'ERP',
      type: 'custom',
      align: 'center',
      minWidth: 90,
      defaultOrder: 90,
    },
    {
      key: 'actions',
      label: 'Opcoes',
      type: 'custom',
      minWidth: 150,
      align: 'center',
      defaultOrder: 1000,
    },
  ])

  return { allTableColumns, filterDefinitions }
}
