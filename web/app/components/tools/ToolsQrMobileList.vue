<script setup lang="ts">
import { computed } from 'vue'

import ToolsQrThumb from '~/components/tools/ToolsQrThumb.vue'
import type { QrCodeItem } from '~/types/tools'

// QR Codes no mobile: um card por QR com a miniatura (toque abre o QR grande),
// slug e destino sempre visiveis. Os demais campos seguem os "Campos do card".
const props = defineProps<{
  rows: QrCodeItem[]
  visibleKeys: string[]
  canManage: boolean
  savingMap: Record<string, boolean>
  emptyText: string
}>()
const emit = defineEmits<{
  view: [id: string]
  download: [item: QrCodeItem]
  edit: [item: QrCodeItem]
  remove: [item: QrCodeItem]
  toggle: [id: string, value: boolean]
}>()

const show = computed(() => new Set(props.visibleKeys))

const showSwitch = computed(() => props.canManage && show.value.has('isActive'))

function metaParts(row: QrCodeItem) {
  const parts: string[] = []
  if (show.value.has('scanCount')) parts.push(`${row.scanCount} scan(s)`)
  if (show.value.has('lastScannedAt') && row.lastScannedAt) {
    parts.push(`ultimo ${dateLabel(row.lastScannedAt)}`)
  }
  if (show.value.has('createdAt')) parts.push(`criado ${dateLabel(row.createdAt)}`)
  if (show.value.has('clientName') && row.clientName) parts.push(row.clientName)
  return parts
}

function dateLabel(value: string) {
  const date = new Date(value)
  return value && Number.isFinite(date.getTime())
    ? date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: '2-digit' })
    : '-'
}
</script>

<template>
  <ul class="tools-cards">
    <li v-if="!rows.length" class="tools-cards__empty">{{ emptyText }}</li>
    <li
      v-for="row in rows"
      :key="row.id"
      class="tools-card tools-card--thumb"
      :class="{ 'is-off': !row.isActive }"
    >
      <ToolsQrThumb
        :slug="row.slug"
        :qr-url="row.qrUrl"
        :fill-color="row.fillColor"
        :back-color="row.backColor"
        :is-active="row.isActive"
        @view="emit('view', row.id)"
        @download="emit('download', row)"
      />
      <div class="tools-card__body">
        <a
          class="tools-card__title tools-card__link"
          :href="row.qrUrl"
          target="_blank"
          rel="noopener noreferrer"
          :title="`Testar ${row.qrUrl} (conta como scan)`"
        >
          {{ row.slug }}
        </a>
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
            :disabled="Boolean(savingMap[`${row.id}:toggle`])"
            @update:model-value="emit('toggle', row.id, Boolean($event))"
          />
        </div>
      </div>
      <div v-if="canManage" class="tools-card__actions">
        <UButton
          icon="i-lucide-pencil"
          color="neutral"
          variant="ghost"
          size="sm"
          :aria-label="`Editar QR ${row.slug}`"
          @click="emit('edit', row)"
        />
        <UButton
          icon="i-lucide-trash-2"
          color="error"
          variant="ghost"
          size="sm"
          :aria-label="`Excluir QR ${row.slug}`"
          :loading="Boolean(savingMap[`${row.id}:delete`])"
          @click="emit('remove', row)"
        />
      </div>
    </li>
  </ul>
</template>

<style scoped src="./tools-cards.css"></style>
<style scoped>
.tools-card__link {
  text-decoration: none;
}

.tools-card__link:active {
  text-decoration: underline;
}

.tools-card__title {
  font-size: 1.05rem;
  overflow-wrap: anywhere;
  white-space: normal;
}
</style>
