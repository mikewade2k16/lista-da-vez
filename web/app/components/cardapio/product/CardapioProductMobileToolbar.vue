<script setup lang="ts">
import OmniTableColumnsConfig from '~/components/omni/table/OmniTableColumnsConfig.vue'
import type { OmniTableColumn } from '~/types/omni/collection'

// Barra da aba Produtos no mobile: so icones, em duas linhas finas.
// 1) busca + novo produto; 2) foto, filtros, campos do card, mais opcoes e,
// com selecao, acoes em lote. Mesmo estado/filtros da versao desktop.
interface PhotoChip {
  value: string
  label: string
  count: number
}

defineProps<{
  photoChips: PhotoChip[]
  photoValue: string
  activeFilterCount: number
  columns: OmniTableColumn[]
  excludeKeys: string[]
  moreItems: unknown[][]
  bulkItems: unknown[][]
  selectedCount: number
  busy: boolean
}>()
const search = defineModel<string>('search', { required: true })
const filtersOpen = defineModel<boolean>('filtersOpen', { required: true })
const visibleColumns = defineModel<string[]>('visibleColumns', { required: true })
const emit = defineEmits<{ create: []; photo: [value: string] }>()

const PHOTO_ICONS: Record<string, string> = {
  '': 'i-lucide-list',
  without: 'i-lucide-image-off',
  with: 'i-lucide-image',
}
</script>

<template>
  <div class="cardapio-mtool">
    <div class="cardapio-mtool__row">
      <UInput
        v-model="search"
        icon="i-lucide-search"
        placeholder="Buscar produto"
        class="cardapio-mtool__search"
        :ui="{ base: search ? 'pr-8' : '' }"
      >
        <template v-if="search" #trailing>
          <UButton
            icon="i-lucide-x"
            color="neutral"
            variant="link"
            size="xs"
            aria-label="Limpar busca"
            @click="search = ''"
          />
        </template>
      </UInput>
      <UButton
        icon="i-lucide-plus"
        color="primary"
        aria-label="Novo produto"
        title="Novo produto"
        @click="emit('create')"
      />
    </div>
    <div class="cardapio-mtool__row">
      <div class="cardapio-mtool__photo" role="group" aria-label="Filtrar por foto">
        <button
          v-for="chip in photoChips"
          :key="chip.value"
          type="button"
          :class="{
            'is-active': photoValue === chip.value,
            'is-warn': chip.value === 'without' && chip.count > 0,
          }"
          :aria-pressed="photoValue === chip.value"
          :aria-label="`${chip.label}: ${chip.count}`"
          :title="chip.label"
          @click="emit('photo', chip.value)"
        >
          <UIcon :name="PHOTO_ICONS[chip.value] || 'i-lucide-list'" />
          <strong>{{ chip.count }}</strong>
        </button>
      </div>
      <div class="cardapio-mtool__icons">
        <UDropdownMenu v-if="selectedCount" :items="bulkItems">
          <UButton
            icon="i-lucide-list-checks"
            :label="String(selectedCount)"
            color="primary"
            variant="soft"
            :aria-label="`Acoes para ${selectedCount} selecionado(s)`"
          />
        </UDropdownMenu>
        <UChip :show="activeFilterCount > 0" :text="activeFilterCount" size="lg">
          <UButton
            icon="i-lucide-sliders-horizontal"
            color="neutral"
            :variant="filtersOpen ? 'soft' : 'ghost'"
            aria-label="Filtros"
            title="Filtros"
            @click="filtersOpen = !filtersOpen"
          />
        </UChip>
        <OmniTableColumnsConfig
          v-model="visibleColumns"
          :columns="columns"
          :exclude-keys="excludeKeys"
          label=""
          title="Campos do card"
        />
        <UDropdownMenu :items="moreItems" :content="{ align: 'end' }">
          <UButton
            icon="i-lucide-ellipsis"
            color="neutral"
            variant="ghost"
            aria-label="Mais opcoes"
            :loading="busy"
          />
        </UDropdownMenu>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cardapio-mtool {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.cardapio-mtool__row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.cardapio-mtool__search {
  flex: 1;
  min-width: 0;
}

.cardapio-mtool__photo {
  display: inline-flex;
  padding: 0.12rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.cardapio-mtool__photo button {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.28rem 0.45rem;
  border: none;
  border-radius: calc(var(--radius-sm) - 2px);
  background: transparent;
  color: var(--text-muted);
  font-size: 0.78rem;
  cursor: pointer;
}

.cardapio-mtool__photo strong {
  color: var(--text-main);
}

.cardapio-mtool__photo button.is-warn strong {
  color: rgb(var(--danger));
}

.cardapio-mtool__photo button.is-active {
  background: rgb(var(--primary) / 0.16);
  color: var(--text-main);
}

.cardapio-mtool__icons {
  display: flex;
  align-items: center;
  gap: 0.1rem;
  margin-left: auto;
}

/* Gatilho do menu de colunas compartilhado vira so icone. */
.cardapio-mtool__icons :deep(.omni-table-columns-config__trigger) {
  background: transparent;
}
</style>
