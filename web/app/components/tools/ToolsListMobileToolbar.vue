<script setup lang="ts">
import OmniTableColumnsConfig from '~/components/omni/table/OmniTableColumnsConfig.vue'
import ToolsStatusChips from '~/components/tools/ToolsStatusChips.vue'
import type { OmniTableColumn } from '~/types/omni/collection'

// Barra do encurtador e do QR no mobile (mesmo modelo do cardapio): so icones,
// em duas linhas finas. 1) busca + criar; 2) status e campos do card.
defineProps<{
  placeholder: string
  createLabel: string
  canCreate: boolean
  total: number
  active: number
  columns: OmniTableColumn[]
  excludeKeys: string[]
}>()
const search = defineModel<string>('search', { required: true })
const status = defineModel<string>('status', { required: true })
const visibleColumns = defineModel<string[]>('visibleColumns', { required: true })
const emit = defineEmits<{ create: [] }>()
</script>

<template>
  <div class="tools-mtool">
    <div class="tools-mtool__row">
      <UInput
        v-model="search"
        icon="i-lucide-search"
        :placeholder="placeholder"
        class="tools-mtool__search"
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
        v-if="canCreate"
        icon="i-lucide-plus"
        color="primary"
        :aria-label="createLabel"
        :title="createLabel"
        @click="emit('create')"
      />
    </div>
    <div class="tools-mtool__row">
      <ToolsStatusChips v-model="status" :total="total" :active="active" compact />
      <div class="tools-mtool__icons">
        <OmniTableColumnsConfig
          v-model="visibleColumns"
          :columns="columns"
          :exclude-keys="excludeKeys"
          label=""
          title="Campos do card"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.tools-mtool {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.tools-mtool__row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.tools-mtool__search {
  flex: 1;
  min-width: 0;
}

.tools-mtool__icons {
  display: flex;
  align-items: center;
  gap: 0.1rem;
  margin-left: auto;
}

.tools-mtool__icons :deep(.omni-table-columns-config__trigger) {
  background: transparent;
}
</style>
