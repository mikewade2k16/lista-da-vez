<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'

import type { ShortLinkItem } from '~/types/tools'

// Visualizacao padrao do encurtador (desktop): grade de cards. O link curto e
// um link de verdade (abre em nova aba), com botoes de copiar e abrir ao lado;
// o destino fica numa linha so. Editar destino, renomear e excluir no "...".
const props = defineProps<{
  rows: ShortLinkItem[]
  canManage: boolean
  savingMap: Record<string, boolean>
  emptyText: string
}>()
const emit = defineEmits<{
  copy: [item: ShortLinkItem]
  open: [url: string]
  editTarget: [item: ShortLinkItem]
  rename: [item: ShortLinkItem]
  remove: [item: ShortLinkItem]
  toggle: [id: string, value: boolean]
}>()

function shortLabel(url: string) {
  return String(url ?? '').replace(/^https?:\/\//i, '')
}

function menuItems(row: ShortLinkItem): DropdownMenuItem[][] {
  const items: DropdownMenuItem[] = [
    { label: 'Abrir destino', icon: 'i-lucide-link', onSelect: () => emit('open', row.targetUrl) },
  ]
  if (props.canManage) {
    items.push(
      { label: 'Editar destino', icon: 'i-lucide-pencil', onSelect: () => emit('editTarget', row) },
      { label: 'Renomear slug', icon: 'i-lucide-pencil-line', onSelect: () => emit('rename', row) },
      {
        label: 'Excluir',
        icon: 'i-lucide-trash-2',
        color: 'error',
        onSelect: () => emit('remove', row),
      },
    )
  }
  return [items]
}
</script>

<template>
  <div class="tools-grid tools-grid--wide">
    <p v-if="!rows.length" class="tools-grid__empty">{{ emptyText }}</p>
    <article
      v-for="row in rows"
      :key="row.id"
      class="tools-grid__card"
      :class="{ 'is-off': !row.isActive }"
    >
      <div class="tools-link-head">
        <a
          class="tools-grid__title"
          :href="row.shortUrl"
          target="_blank"
          rel="noopener noreferrer"
          :title="`Abrir ${row.shortUrl}`"
        >
          <span>{{ shortLabel(row.shortUrl) }}</span>
        </a>
        <UButton
          icon="i-lucide-copy"
          color="neutral"
          variant="soft"
          size="xs"
          title="Copiar link curto"
          :aria-label="`Copiar ${row.shortUrl}`"
          @click="emit('copy', row)"
        />
        <UButton
          icon="i-lucide-external-link"
          color="neutral"
          variant="soft"
          size="xs"
          title="Abrir link curto"
          :aria-label="`Abrir ${row.shortUrl}`"
          @click="emit('open', row.shortUrl)"
        />
      </div>
      <p class="tools-grid__sub" :title="row.targetUrl">{{ row.targetUrl }}</p>

      <footer class="tools-grid__foot">
        <span class="tools-grid__stat">{{ row.hits }} clique(s)</span>
        <USwitch
          v-if="canManage"
          size="sm"
          :aria-label="row.isActive ? `Desativar ${row.slug}` : `Ativar ${row.slug}`"
          :title="row.isActive ? 'Ativo' : 'Inativo'"
          :model-value="row.isActive"
          :disabled="Boolean(savingMap[`${row.id}:isActive`])"
          @update:model-value="emit('toggle', row.id, Boolean($event))"
        />
        <span v-else-if="!row.isActive" class="tools-grid__off">Inativo</span>
        <div class="tools-grid__actions">
          <UDropdownMenu :items="menuItems(row)" :content="{ align: 'end' }">
            <UButton
              icon="i-lucide-ellipsis"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Mais opcoes"
              :aria-label="`Opcoes de ${row.slug}`"
              :loading="Boolean(savingMap[`${row.id}:delete`] || savingMap[`${row.id}:slug`])"
            />
          </UDropdownMenu>
        </div>
      </footer>
    </article>
  </div>
</template>

<style scoped src="./tools-grid.css"></style>
<style scoped>
.tools-link-head {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  min-width: 0;
}

.tools-link-head .tools-grid__title {
  flex: 1;
}
</style>
