<script setup lang="ts">
import { computed } from 'vue'

import type { DropdownMenuItem } from '@nuxt/ui'

import type { ShortLinkItem } from '~/types/tools'

// Links curtos no mobile: um card por link com o link curto e o destino sempre
// visiveis; toque no link curto abre, o botao ao lado copia. Os demais campos seguem os "Campos do
// card". Demais acoes (abrir, editar destino, renomear, excluir) ficam no menu "...".
const props = defineProps<{
  rows: ShortLinkItem[]
  visibleKeys: string[]
  canManage: boolean
  savingMap: Record<string, boolean>
  emptyText: string
}>()
const emit = defineEmits<{
  copy: [item: ShortLinkItem]
  open: [url: string]
  rename: [item: ShortLinkItem]
  editTarget: [item: ShortLinkItem]
  remove: [item: ShortLinkItem]
  toggle: [id: string, value: boolean]
}>()

const show = computed(() => new Set(props.visibleKeys))
const showSwitch = computed(() => props.canManage && show.value.has('isActive'))

function dateLabel(value: string) {
  const date = new Date(value)
  return value && Number.isFinite(date.getTime())
    ? date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: '2-digit' })
    : '-'
}

function shortLabel(url: string) {
  return String(url ?? '').replace(/^https?:\/\//i, '')
}

function metaParts(row: ShortLinkItem) {
  const parts: string[] = []
  if (show.value.has('hits')) parts.push(`${row.hits} clique(s)`)
  if (show.value.has('lastHitAt') && row.lastHitAt) parts.push(`ultimo ${dateLabel(row.lastHitAt)}`)
  if (show.value.has('createdAt')) parts.push(`criado ${dateLabel(row.createdAt)}`)
  if (show.value.has('clientName') && row.clientName) parts.push(row.clientName)
  return parts
}

function menuItems(row: ShortLinkItem): DropdownMenuItem[][] {
  const items: DropdownMenuItem[] = [
    {
      label: 'Abrir link curto',
      icon: 'i-lucide-external-link',
      onSelect: () => emit('open', row.shortUrl),
    },
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
  <ul class="tools-cards">
    <li v-if="!rows.length" class="tools-cards__empty">{{ emptyText }}</li>
    <li v-for="row in rows" :key="row.id" class="tools-card" :class="{ 'is-off': !row.isActive }">
      <div class="tools-card__body">
        <div class="tools-card__head">
          <a
            class="tools-card__title tools-card__link"
            :href="row.shortUrl"
            target="_blank"
            rel="noopener noreferrer"
            :title="`Abrir ${row.shortUrl}`"
          >
            {{ shortLabel(row.shortUrl) }}
          </a>
          <UButton
            icon="i-lucide-copy"
            color="neutral"
            variant="ghost"
            size="xs"
            :aria-label="`Copiar ${row.shortUrl}`"
            @click="emit('copy', row)"
          />
        </div>
        <p class="tools-card__meta">
          <span>{{ row.targetUrl }}</span>
        </p>
        <p v-if="metaParts(row).length || (!row.isActive && !showSwitch)" class="tools-card__meta">
          <span>{{ metaParts(row).join(' · ') }}</span>
          <span v-if="!row.isActive && !showSwitch" class="tools-card__off">Inativo</span>
        </p>
        <div v-if="showSwitch" class="tools-card__toggles">
          <USwitch
            size="sm"
            label="Ativo"
            :model-value="row.isActive"
            :disabled="Boolean(savingMap[`${row.id}:isActive`])"
            @update:model-value="emit('toggle', row.id, Boolean($event))"
          />
        </div>
      </div>
      <div class="tools-card__actions">
        <UDropdownMenu :items="menuItems(row)" :content="{ align: 'end' }">
          <UButton
            icon="i-lucide-ellipsis-vertical"
            color="neutral"
            variant="ghost"
            size="sm"
            :aria-label="`Opcoes de ${row.slug}`"
            :loading="Boolean(savingMap[`${row.id}:delete`] || savingMap[`${row.id}:slug`])"
          />
        </UDropdownMenu>
      </div>
    </li>
  </ul>
</template>

<style scoped src="./tools-cards.css"></style>
<style scoped>
.tools-card__head {
  display: flex;
  align-items: center;
  gap: 0.15rem;
  min-width: 0;
}

.tools-card__link {
  min-width: 0;
  text-decoration: none;
}
</style>
