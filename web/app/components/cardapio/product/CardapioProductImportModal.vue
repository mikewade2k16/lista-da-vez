<script setup lang="ts">
import { computed, ref } from 'vue'

import {
  applyProductImportCategoryDecisions,
  detectProductTransferFormat,
  parseProductTransfer,
  type ProductImportCategoryDecision,
  type ProductImportInput,
  type ProductImportResult,
  type ProductTransferDocument,
} from '~/domain/cardapio/product-transfer'
import { slugify } from '~/domain/utils/slugify'
import { useCardapioStore } from '~/stores/cardapio'
import { getApiErrorMessage } from '~/utils/api-client'

interface CategoryDraft extends ProductImportCategoryDecision {
  productCount: number
}

const props = defineProps<{
  restaurantId: string
  importing: boolean
  result: ProductImportResult | null
}>()

const emit = defineEmits<{
  close: []
  reset: []
  submit: [input: ProductImportInput]
}>()

const store = useCardapioStore()
const document = ref<ProductTransferDocument | null>(null)
const fileName = ref('')
const errorMessage = ref('')
const updateExisting = ref(true)
const previewing = ref(false)
const categoriesReviewed = ref(false)
const categoryDrafts = ref<CategoryDraft[]>([])
const selectedCategorySlugs = ref<string[]>([])

const activeCategories = computed(() => categoryDrafts.value.filter((item) => !item.removed))
const removedCategories = computed(() => categoryDrafts.value.filter((item) => item.removed))
const removedProductCount = computed(() =>
  removedCategories.value.reduce((total, item) => total + item.productCount, 0),
)
const allActiveSelected = computed(
  () =>
    activeCategories.value.length > 0 &&
    activeCategories.value.every((item) => selectedCategorySlugs.value.includes(item.originalSlug)),
)
const categoryNamesValid = computed(() =>
  activeCategories.value.every((item) => Boolean(item.name.trim())),
)
const hasCategoryEdits = computed(() =>
  categoryDrafts.value.some((item) => item.name.trim() !== item.originalName.trim()),
)
const canImport = computed(
  () =>
    Boolean(document.value?.products.length) &&
    !props.importing &&
    !previewing.value &&
    !errorMessage.value &&
    categoriesReviewed.value &&
    categoryNamesValid.value,
)
const reviewButtonLabel = computed(() => {
  if (!activeCategories.value.length) return 'Confirmar produtos sem categoria'
  if (removedCategories.value.length || hasCategoryEdits.value) {
    return 'Confirmar categorias revisadas'
  }
  return `Aceitar ${activeCategories.value.length} como estão`
})

async function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  emit('reset')
  document.value = null
  errorMessage.value = ''
  fileName.value = file?.name ?? ''
  categoryDrafts.value = []
  selectedCategorySlugs.value = []
  categoriesReviewed.value = false
  if (!file) return
  if (file.size > 10 * 1024 * 1024) {
    errorMessage.value = 'O arquivo excede o limite de 10 MB.'
    return
  }
  try {
    const format = detectProductTransferFormat(file.name)
    const parsed = parseProductTransfer(await file.text(), format)
    document.value = parsed
    await loadCategoryPreview(parsed)
  } catch (caught) {
    errorMessage.value = getApiErrorMessage(caught, 'Não foi possível analisar o arquivo.')
  }
}

async function loadCategoryPreview(parsed: ProductTransferDocument) {
  previewing.value = true
  try {
    const preview = await store.previewProductImport(props.restaurantId, {
      updateExisting: updateExisting.value,
      acceptedCategorySlugs: [],
      products: parsed.products,
    })
    categoryDrafts.value = preview.newCategories.map((category) => ({
      originalSlug: category.slug,
      originalName: category.name,
      name: category.name,
      productCount: category.productCount,
      removed: false,
    }))
    categoriesReviewed.value = preview.newCategories.length === 0
  } finally {
    previewing.value = false
  }
}

function setCategorySelected(slug: string, value: boolean | 'indeterminate') {
  const selected = new Set(selectedCategorySlugs.value)
  if (value === true) selected.add(slug)
  else selected.delete(slug)
  selectedCategorySlugs.value = [...selected]
}

function setAllSelected(value: boolean | 'indeterminate') {
  selectedCategorySlugs.value =
    value === true ? activeCategories.value.map((item) => item.originalSlug) : []
}

function markReviewPending() {
  categoriesReviewed.value = false
}

function removeCategory(slug: string) {
  const category = categoryDrafts.value.find((item) => item.originalSlug === slug)
  if (!category) return
  category.removed = true
  setCategorySelected(slug, false)
  markReviewPending()
}

function removeSelectedCategories() {
  const selected = new Set(selectedCategorySlugs.value)
  categoryDrafts.value.forEach((item) => {
    if (selected.has(item.originalSlug)) item.removed = true
  })
  selectedCategorySlugs.value = []
  markReviewPending()
}

function restoreRemovedCategories() {
  categoryDrafts.value.forEach((item) => {
    item.removed = false
  })
  markReviewPending()
}

function existingCategoryName(category: CategoryDraft): string {
  const name = category.name.trim()
  const targetSlug =
    name === category.originalName.trim() ? slugify(category.originalSlug) : slugify(name)
  return store.categories.find((item) => slugify(item.slug) === targetSlug)?.name ?? ''
}

function confirmCategories() {
  if (!categoryNamesValid.value) return
  categoriesReviewed.value = true
}

function submit() {
  if (!document.value || !canImport.value) return
  const categoryResult = applyProductImportCategoryDecisions(
    document.value.products,
    categoryDrafts.value,
  )
  emit('submit', {
    updateExisting: updateExisting.value,
    ...categoryResult,
  })
}
</script>

<template>
  <UModal
    :open="true"
    title="Importar produtos"
    description="Envie JSON ou CSV. Antes de gravar, revise todas as categorias que ainda não existem."
    :ui="{ content: 'max-w-4xl' }"
    @update:open="!$event && emit('close')"
  >
    <template #body>
      <div class="cardapio-product-import">
        <label class="cardapio-product-import__drop">
          <span class="cardapio-product-import__drop-title">Selecionar arquivo JSON ou CSV</span>
          <span class="cardapio-product-import__drop-hint">
            Até 10 MB. Nenhuma categoria nova será criada sem sua confirmação.
          </span>
          <input
            class="cardapio-product-import__file"
            type="file"
            accept=".json,.csv,application/json,text/csv"
            :disabled="importing || previewing"
            @change="onFileChange"
          />
        </label>

        <UAlert
          v-if="errorMessage"
          color="error"
          variant="soft"
          icon="i-lucide-alert-triangle"
          title="Arquivo inválido"
          :description="errorMessage"
        />

        <div v-if="document" class="cardapio-product-import__summary">
          <div>
            <strong>{{ document.products.length }}</strong>
            produto(s) encontrados
          </div>
          <span>{{ fileName }}</span>
        </div>

        <div v-if="previewing" class="cardapio-product-import__loading">
          <UIcon name="i-lucide-loader-circle" class="animate-spin" />
          Conferindo categorias existentes...
        </div>

        <section
          v-else-if="document && categoryDrafts.length"
          class="cardapio-product-import__categories"
        >
          <UAlert
            color="warning"
            variant="soft"
            icon="i-lucide-tags"
            :title="`${categoryDrafts.length} categoria(s) nova(s) encontrada(s)`"
            description="Revise os nomes. Remover uma categoria fará seus produtos entrarem sem categoria."
          />

          <div class="cardapio-product-import__category-tools">
            <UCheckbox
              :model-value="allActiveSelected"
              label="Selecionar todas"
              @update:model-value="setAllSelected"
            />
            <UButton
              label="Remover selecionadas"
              icon="i-lucide-trash-2"
              color="error"
              variant="soft"
              size="sm"
              :disabled="!selectedCategorySlugs.length"
              @click="removeSelectedCategories"
            />
          </div>

          <div class="cardapio-product-import__category-list">
            <div
              v-for="category in activeCategories"
              :key="category.originalSlug"
              class="cardapio-product-import__category"
            >
              <UCheckbox
                :model-value="selectedCategorySlugs.includes(category.originalSlug)"
                :aria-label="`Selecionar ${category.name}`"
                @update:model-value="setCategorySelected(category.originalSlug, $event)"
              />
              <div class="cardapio-product-import__category-main">
                <UInput
                  v-model="category.name"
                  aria-label="Nome da categoria"
                  :color="category.name.trim() ? 'neutral' : 'error'"
                  :disabled="importing"
                  @update:model-value="markReviewPending"
                />
                <span>
                  {{ category.productCount }} produto(s)
                  <template v-if="existingCategoryName(category)">
                    · usará a categoria existente “{{ existingCategoryName(category) }}”
                  </template>
                </span>
              </div>
              <UButton
                icon="i-lucide-trash-2"
                color="error"
                variant="ghost"
                size="sm"
                title="Importar estes produtos sem categoria"
                :aria-label="`Remover categoria ${category.name}`"
                @click="removeCategory(category.originalSlug)"
              />
            </div>
          </div>

          <UAlert
            v-if="removedCategories.length"
            color="neutral"
            variant="soft"
            :title="`${removedCategories.length} categoria(s) removida(s)`"
            :description="`${removedProductCount} produto(s) serão importados sem categoria.`"
          >
            <template #actions>
              <UButton
                label="Restaurar removidas"
                color="neutral"
                variant="ghost"
                size="xs"
                @click="restoreRemovedCategories"
              />
            </template>
          </UAlert>

          <div class="cardapio-product-import__review-action">
            <UBadge v-if="categoriesReviewed" color="success" variant="soft">
              Categorias confirmadas
            </UBadge>
            <UButton
              v-else
              :label="reviewButtonLabel"
              icon="i-lucide-check-check"
              color="primary"
              :disabled="!categoryNamesValid"
              @click="confirmCategories"
            />
          </div>
        </section>

        <UAlert
          v-else-if="document && !previewing"
          color="success"
          variant="soft"
          icon="i-lucide-circle-check"
          title="Nenhuma categoria nova"
          description="Todas as categorias do arquivo já existem ou os produtos estão sem categoria."
        />

        <UCheckbox
          v-model="updateExisting"
          label="Atualizar produtos existentes com o mesmo slug"
          description="Desmarcado: produtos existentes são preservados e contabilizados como ignorados."
          :disabled="importing"
        />

        <UAlert
          v-if="result"
          :color="result.failed ? 'warning' : 'success'"
          variant="soft"
          :icon="result.failed ? 'i-lucide-triangle-alert' : 'i-lucide-circle-check'"
          title="Resultado da importação"
          :description="`${result.created} criado(s), ${result.updated} atualizado(s), ${result.skipped} ignorado(s) e ${result.failed} com erro.`"
        />

        <ul v-if="result?.errors.length" class="cardapio-product-import__errors">
          <li v-for="item in result.errors.slice(0, 8)" :key="`${item.row}:${item.slug}`">
            Linha {{ item.row }}
            <template v-if="item.slug">({{ item.slug }})</template>
            :
            {{ item.message }}
          </li>
          <li v-if="result.errors.length > 8">E mais {{ result.errors.length - 8 }} erro(s).</li>
        </ul>
      </div>
    </template>

    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton
          label="Fechar"
          color="neutral"
          variant="ghost"
          :disabled="importing"
          @click="emit('close')"
        />
        <UButton
          label="Importar"
          icon="i-lucide-upload"
          color="primary"
          :loading="importing"
          :disabled="!canImport"
          @click="submit"
        />
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.cardapio-product-import {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.cardapio-product-import__drop {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 1rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2) / 0.45);
}

.cardapio-product-import__drop-title {
  color: var(--text-main);
  font-weight: 650;
}

.cardapio-product-import__drop-hint,
.cardapio-product-import__summary span,
.cardapio-product-import__category-main span {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.cardapio-product-import__file {
  margin-top: 0.35rem;
  color: var(--text-main);
}

.cardapio-product-import__summary,
.cardapio-product-import__category-tools,
.cardapio-product-import__category,
.cardapio-product-import__review-action,
.cardapio-product-import__loading {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.cardapio-product-import__summary,
.cardapio-product-import__category-tools {
  justify-content: space-between;
}

.cardapio-product-import__summary {
  padding: 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  color: var(--text-main);
}

.cardapio-product-import__categories {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.cardapio-product-import__category-list {
  display: flex;
  max-height: 20rem;
  flex-direction: column;
  gap: 0.5rem;
  overflow-y: auto;
  padding-right: 0.25rem;
}

.cardapio-product-import__category {
  padding: 0.65rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.35);
}

.cardapio-product-import__category-main {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 0.25rem;
}

.cardapio-product-import__review-action {
  justify-content: flex-end;
}

.cardapio-product-import__loading {
  justify-content: center;
  padding: 1rem;
  color: var(--text-muted);
}

.cardapio-product-import__errors {
  max-height: 10rem;
  overflow-y: auto;
  padding-left: 1.25rem;
  color: var(--text-muted);
  font-size: 0.8rem;
}
</style>
