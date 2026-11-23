<script setup lang="ts">
import ToolsQrThumb from '~/components/tools/ToolsQrThumb.vue'
import type { QrCodeItem } from '~/types/tools'

// Visualizacao padrao do QR (desktop): grade de cards com o QR grande (clique
// abre em tela cheia), o slug em destaque como link de teste (abre o /q/{slug}
// rastreado, entao conta como scan) e o destino numa linha so.
defineProps<{
  rows: QrCodeItem[]
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
</script>

<template>
  <div class="tools-grid">
    <p v-if="!rows.length" class="tools-grid__empty">{{ emptyText }}</p>
    <article
      v-for="row in rows"
      :key="row.id"
      class="tools-grid__card"
      :class="{ 'is-off': !row.isActive }"
    >
      <div class="tools-grid__media">
        <ToolsQrThumb
          :slug="row.slug"
          :qr-url="row.qrUrl"
          :fill-color="row.fillColor"
          :back-color="row.backColor"
          :is-active="row.isActive"
          :size="148"
          @view="emit('view', row.id)"
          @download="emit('download', row)"
        />
      </div>

      <a
        class="tools-grid__title"
        :href="row.qrUrl"
        target="_blank"
        rel="noopener noreferrer"
        :title="`Testar ${row.qrUrl} (conta como scan)`"
      >
        <span>{{ row.slug }}</span>
        <UIcon name="i-lucide-external-link" class="tools-grid__title-icon" />
      </a>
      <p class="tools-grid__sub" :title="row.targetUrl">{{ row.targetUrl }}</p>

      <footer class="tools-grid__foot">
        <span class="tools-grid__stat">{{ row.scanCount }} scan(s)</span>
        <USwitch
          v-if="canManage"
          size="sm"
          :aria-label="row.isActive ? `Desativar ${row.slug}` : `Ativar ${row.slug}`"
          :title="row.isActive ? 'Ativo' : 'Inativo'"
          :model-value="row.isActive"
          :disabled="Boolean(savingMap[`${row.id}:toggle`])"
          @update:model-value="emit('toggle', row.id, Boolean($event))"
        />
        <span v-else-if="!row.isActive" class="tools-grid__off">Inativo</span>
        <div class="tools-grid__actions">
          <UButton
            icon="i-lucide-download"
            color="neutral"
            variant="ghost"
            size="xs"
            title="Baixar PNG"
            :aria-label="`Baixar QR ${row.slug}`"
            @click="emit('download', row)"
          />
          <template v-if="canManage">
            <UButton
              icon="i-lucide-pencil"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Editar QR"
              :aria-label="`Editar QR ${row.slug}`"
              @click="emit('edit', row)"
            />
            <UButton
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="xs"
              title="Excluir QR"
              :aria-label="`Excluir QR ${row.slug}`"
              :loading="Boolean(savingMap[`${row.id}:delete`])"
              @click="emit('remove', row)"
            />
          </template>
        </div>
      </footer>
    </article>
  </div>
</template>

<style scoped src="./tools-grid.css"></style>
<style scoped>
.tools-grid__title {
  font-size: 1.1rem;
}

.tools-grid__title > span {
  overflow-wrap: anywhere;
  white-space: normal;
}

.tools-grid__media {
  display: flex;
  justify-content: center;
  padding-bottom: 0.35rem;
}
</style>
