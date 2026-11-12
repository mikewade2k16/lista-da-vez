import { computed, ref } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import {
  serializeProductTransfer,
  type ProductImportInput,
  type ProductImportResult,
} from '~/domain/cardapio/product-transfer'

// Menu secundario da aba Produtos: importacao/exportacao do catalogo e a
// preferencia de digitacao de preco (restaurants.settings, PATCH canonico).
export function useCardapioProductTransfer(productCount: () => number, onImported: () => void) {
  const store = useCardapioStore()
  const ui = useUiStore()

  const importOpen = ref(false)
  const importBusy = ref(false)
  const importResult = ref<ProductImportResult | null>(null)
  const exportBusy = ref(false)
  const settingsBusy = ref(false)
  const wholeAmounts = computed(() => store.restaurant?.settings.productPriceDecimals !== true)

  async function setWholeAmounts(enabled: boolean) {
    const restaurant = store.restaurant
    if (!restaurant || settingsBusy.value) return
    settingsBusy.value = true
    try {
      await store.patchRestaurant(restaurant.id, {
        settings: { ...restaurant.settings, productPriceDecimals: !enabled },
      })
      ui.success('Preferencia de valores salva para este cardapio.')
    } catch (caught) {
      ui.error(getApiErrorMessage(caught, 'Nao foi possivel salvar a preferencia de valores.'))
    } finally {
      settingsBusy.value = false
    }
  }

  async function exportFile(format: 'json' | 'csv') {
    const restaurantId = store.restaurantId
    if (!restaurantId || exportBusy.value || import.meta.server) return
    exportBusy.value = true
    try {
      const document = await store.exportProducts(restaurantId)
      const content = serializeProductTransfer(document, format)
      const mime = format === 'json' ? 'application/json;charset=utf-8' : 'text/csv;charset=utf-8'
      const prefix = format === 'csv' ? '﻿' : ''
      const blob = new Blob([prefix, content], { type: mime })
      const url = URL.createObjectURL(blob)
      const link = window.document.createElement('a')
      link.href = url
      link.download = `produtos-${store.restaurant?.slug || restaurantId}.${format}`
      link.click()
      URL.revokeObjectURL(url)
      ui.success(`${document.products.length} produto(s) exportado(s).`)
    } catch (caught) {
      ui.error(getApiErrorMessage(caught, 'Nao foi possivel exportar os produtos.'))
    } finally {
      exportBusy.value = false
    }
  }

  function openImport() {
    importResult.value = null
    importOpen.value = true
  }

  function closeImport() {
    if (importBusy.value) return
    importOpen.value = false
  }

  async function importProducts(input: ProductImportInput) {
    const restaurantId = store.restaurantId
    if (!restaurantId || importBusy.value) return
    importBusy.value = true
    importResult.value = null
    try {
      const result = await store.importProducts(restaurantId, input)
      importResult.value = result
      onImported()
      if (result.failed) {
        ui.error(`Importacao concluida com ${result.failed} produto(s) rejeitado(s).`)
      } else {
        ui.success(`${result.created + result.updated} produto(s) importado(s).`)
      }
    } catch (caught) {
      ui.error(getApiErrorMessage(caught, 'Nao foi possivel importar os produtos.'))
    } finally {
      importBusy.value = false
    }
  }

  const moreItems = computed(() => [
    [
      { label: 'Importar produtos', icon: 'i-lucide-upload', onSelect: openImport },
      {
        label: 'Exportar JSON',
        icon: 'i-lucide-braces',
        disabled: exportBusy.value || !productCount(),
        onSelect: () => exportFile('json'),
      },
      {
        label: 'Exportar CSV',
        icon: 'i-lucide-sheet',
        disabled: exportBusy.value || !productCount(),
        onSelect: () => exportFile('csv'),
      },
    ],
    [
      {
        label: 'Precos sem centavos (22 = R$ 22)',
        type: 'checkbox' as const,
        checked: wholeAmounts.value,
        disabled: settingsBusy.value || !store.restaurant,
        onUpdateChecked: (checked: boolean) => setWholeAmounts(checked),
        onSelect: (event: Event) => event.preventDefault(),
      },
    ],
  ])

  return {
    importOpen,
    importBusy,
    importResult,
    exportBusy,
    settingsBusy,
    moreItems,
    closeImport,
    importProducts,
  }
}
