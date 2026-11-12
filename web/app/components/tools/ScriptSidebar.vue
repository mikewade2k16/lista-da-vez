<script setup lang="ts">
import { GripVertical, Plus, Trash2 } from 'lucide-vue-next'
import ScriptSortable from '~/components/tools/ScriptSortable.vue'
import AppSearchInput from '~/components/ui/AppSearchInput.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type { ScriptClient, ScriptDocument } from '~/composables/useScriptsManager'

// Lateral dos roteiros: busca + filtro por cliente no topo (sem título, para ganhar altura),
// miniaturas de tamanho fixo e arrastar para ordenar só com a lista completa.
const search = defineModel<string>('search', { required: true })
const clientFilter = defineModel<string>('clientFilter', { required: true })
const props = defineProps<{
  documents: ScriptDocument[]
  clients: ScriptClient[]
  selectedId: string
  accountId: string
  canView: boolean
  canManage: boolean
  loading: boolean
  saving: boolean
  opening: boolean
  hasMore: boolean
}>()
const emit = defineEmits<{
  create: []
  select: [document: ScriptDocument]
  remove: [id: string]
  move: [from: number, to: number]
  loadMore: []
}>()
const filtered = computed(() => Boolean(search.value.trim() || clientFilter.value))
// Só clientes que já têm roteiro; sem nenhum, o filtro nem aparece.
const usedClients = computed(() => props.clients.filter((client) => client.scriptCount > 0))
const clientOptions = computed(() => [
  { value: '', label: 'Todos os clientes' },
  { value: 'none', label: 'Sem cliente' },
  ...usedClients.value.map((client) => ({ value: client.id, label: client.name })),
])
const textParser = import.meta.client ? new DOMParser() : undefined
function plain(html: string) {
  return (
    textParser
      ?.parseFromString(html, 'text/html')
      .body.textContent?.replace(/\s+/g, ' ')
      .trim()
      .slice(0, 160) || ''
  )
}
// List responses carry `preview`; documents just saved carry full `rows` instead.
function thumbRows(document: ScriptDocument) {
  return (document.preview?.length ? document.preview : document.rows || [])
    .slice(0, 3)
    .map((row) => ({ audio: plain(row.audio), video: plain(row.video) }))
}
</script>

<template>
  <aside class="scripts-sidebar">
    <div v-if="canView && accountId" class="scripts-sidebar-tools">
      <div class="scripts-sidebar-row">
        <AppSearchInput
          v-model="search"
          class="scripts-search"
          placeholder="Buscar por título ou texto"
          aria-label="Buscar roteiros"
          :debounce-ms="300"
          compact
        />
        <button
          type="button"
          class="script-icon-button scripts-create"
          title="Criar novo roteiro"
          aria-label="Criar novo roteiro"
          :disabled="!canManage || saving || opening"
          @click="emit('create')"
        >
          <Plus :size="20" />
        </button>
      </div>
      <AppSelectField
        v-if="usedClients.length"
        v-model="clientFilter"
        class="scripts-client-filter"
        :options="clientOptions"
        :show-leading-icon="false"
        compact
        searchable
        placeholder="Todos os clientes"
        search-placeholder="Buscar cliente"
      />
    </div>
    <div class="sidebar-document">
      <p v-if="!canView">Você não tem permissão para acessar os roteiros.</p>
      <p v-else-if="!accountId">Selecione uma conta.</p>
      <p v-else-if="loading && !documents.length">Carregando documentos…</p>
      <p v-else-if="!documents.length && filtered">Nenhum roteiro encontrado com estes filtros.</p>
      <p v-else-if="!documents.length">Nenhum documento salvo.</p>
      <p v-else-if="filtered" class="scripts-search-hint">Limpe os filtros para reordenar.</p>
      <ScriptSortable
        class="script-list"
        handle=".script-drag"
        :disabled="!canManage || saving || loading || filtered"
        @move="(from, to) => emit('move', from, to)"
      >
        <div v-for="document in documents" :key="document.id" class="script-col">
          <div
            class="script-card"
            :class="{ 'script-card--active': selectedId === document.id }"
            role="button"
            tabindex="0"
            @click="emit('select', document)"
            @keydown.enter.prevent="emit('select', document)"
            @keydown.space.prevent="emit('select', document)"
          >
            <div v-if="!document.shared" class="buttons-script-card">
              <button
                type="button"
                class="script-icon-button btn-delete"
                title="Excluir"
                :disabled="!canManage || saving"
                @click.stop="emit('remove', document.id)"
              >
                <Trash2 :size="18" />
              </button>
              <button
                v-if="!filtered"
                type="button"
                class="script-icon-button script-drag"
                title="Reordenar documento"
                :disabled="!canManage || saving"
                @click.stop
              >
                <GripVertical :size="16" />
              </button>
            </div>
            <div class="script-thumb" aria-hidden="true">
              <div class="thumb-title">{{ document.title }}</div>
              <div v-if="document.shared || document.clientName" class="thumb-client">
                {{ document.shared ? 'Compartilhado com você' : document.clientName }}
              </div>
              <div class="thumb-head">
                <span>Áudio</span>
                <span>Vídeo</span>
              </div>
              <div
                v-for="(row, rowIndex) in thumbRows(document)"
                :key="rowIndex"
                class="thumb-rows"
              >
                <p>{{ row.audio }}</p>
                <p>{{ row.video }}</p>
              </div>
            </div>
          </div>
        </div>
      </ScriptSortable>
      <button
        v-if="hasMore"
        type="button"
        class="script-load-more"
        :disabled="loading"
        @click="emit('loadMore')"
      >
        Carregar mais
      </button>
    </div>
  </aside>
</template>
