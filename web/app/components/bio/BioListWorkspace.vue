<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { navigateTo, useRuntimeConfig } from '#app'

import BioCreateModal from '~/components/bio/BioCreateModal.vue'
import { useBioStore } from '~/stores/bio'
import { useTenantsStore } from '~/stores/tenants'
import type { BioListFilters, BioStatus, BioSummary } from '~/domain/bio/types'

// Lista das bios da account (admin ve todas, com filtro por cliente). Tabela:
// nome, slug, cliente, status, atualizado em + busca + criar. O filtro por
// cliente so aparece para platform_admin (accounts via useTenantsStore).

const store = useBioStore()
const tenants = useTenantsStore()

const search = ref('')
// USelect (Reka UI) proibe SelectItem com value vazio; usamos o sentinela 'all'
// e convertemos para '' (sem filtro) em currentFilters.
const statusFilter = ref<BioStatus | 'all'>('all')
const accountFilter = ref('all')
const createOpen = ref(false)
const creating = ref(false)
const createError = ref('')
const deletingId = ref('')
const duplicatingId = ref('')

// Base do front publico da bio (NUXT_PUBLIC_BIO_FRONT_URL). O link "Ver online"
// monta `bioFrontUrl + '/' + slug` (SEM /bio — alinhado ao subagente D).
const config = useRuntimeConfig()
const bioFrontUrl = computed(() =>
  String((config.public as Record<string, unknown>).bioFrontUrl || '')
    .trim()
    .replace(/\/$/, ''),
)

function bioPublicUrl(bio: BioSummary): string {
  if (!bioFrontUrl.value || !bio.slug) {
    return ''
  }
  return `${bioFrontUrl.value}/${bio.slug}`
}

const STATUS_ITEMS = [
  { label: 'Todos os status', value: 'all' },
  { label: 'Rascunho', value: 'draft' },
  { label: 'Publicada', value: 'published' },
]

const isAdmin = computed(() => store.isAdmin)

const accountItems = computed(() => [
  { label: 'Todos os clientes', value: 'all' },
  ...tenants.tenants.map((tenant: { id: string; name: string }) => ({
    label: tenant.name,
    value: tenant.id,
  })),
])

const tenantOptions = computed(() =>
  tenants.tenants.map((tenant: { id: string; name: string }) => ({
    id: tenant.id,
    name: tenant.name,
  })),
)

// Busca cliente-side por nome/slug em cima da lista ja escopada pelo backend
// (q tambem e enviado ao backend; o filtro local cobre digitacao rapida).
const filteredBios = computed<BioSummary[]>(() => {
  const term = search.value.trim().toLowerCase()
  if (!term) {
    return store.bios
  }
  return store.bios.filter(
    (bio) => bio.name.toLowerCase().includes(term) || bio.slug.toLowerCase().includes(term),
  )
})

function currentFilters(): BioListFilters {
  return {
    accountId: isAdmin.value && accountFilter.value !== 'all' ? accountFilter.value : '',
    status: statusFilter.value === 'all' ? '' : statusFilter.value,
    q: search.value.trim(),
  }
}

async function refresh() {
  await store.loadBios(currentFilters())
}

function formatDate(value?: string | null): string {
  if (!value) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '—'
  }
  return date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric' })
}

function statusLabel(status: BioStatus): string {
  return status === 'published' ? 'Publicada' : 'Rascunho'
}

function openEditor(bio: BioSummary) {
  void navigateTo(`/site/bio/${bio.id}`)
}

async function onCreate(payload: { name: string; slug: string; accountId: string; edit: boolean }) {
  creating.value = true
  createError.value = ''
  try {
    const result = await store.createBio(payload)
    if (!result.ok) {
      createError.value = result.message
      return
    }
    createOpen.value = false
    if (payload.edit) {
      // "Criar e editar": abre o editor da nova bio.
      void navigateTo(`/site/bio/${result.bio.id}`)
      return
    }
    // "Criar": volta a lista (recarrega para a nova bio aparecer).
    await refresh()
  } finally {
    creating.value = false
  }
}

async function onDuplicate(bio: BioSummary) {
  if (duplicatingId.value) {
    return
  }
  duplicatingId.value = bio.id
  createError.value = ''
  try {
    const result = await store.duplicateBio(bio.id)
    if (!result.ok) {
      createError.value = result.message
      return
    }
    // Abre o editor da copia (fluxo natural: duplicou para editar).
    void navigateTo(`/site/bio/${result.bio.id}`)
  } finally {
    duplicatingId.value = ''
  }
}

async function onDelete(bio: BioSummary) {
  if (deletingId.value) {
    return
  }
  // Confirmacao simples; evita exclusao acidental sem depender de modal extra.
  const confirmed = typeof window !== 'undefined' && window.confirm(`Excluir a bio "${bio.name}"?`)
  if (!confirmed) {
    return
  }
  deletingId.value = bio.id
  try {
    await store.deleteBio(bio.id)
  } finally {
    deletingId.value = ''
  }
}

watch([statusFilter, accountFilter], () => {
  void refresh()
})

onMounted(async () => {
  if (isAdmin.value) {
    await tenants.ensureLoaded()
  }
  await refresh()
})
</script>

<template>
  <section class="bio-list">
    <header class="bio-list__header">
      <div class="bio-list__title-block">
        <h1 class="bio-list__title">Bio</h1>
        <p class="bio-list__subtitle">Paginas de link-in-bio servidas pelo front publico.</p>
      </div>
      <UButton icon="i-lucide-plus" color="primary" label="Nova bio" @click="createOpen = true" />
    </header>

    <div class="bio-list__filters">
      <UInput
        class="bio-list__search"
        :model-value="search"
        icon="i-lucide-search"
        placeholder="Buscar por nome ou slug"
        @update:model-value="search = String($event ?? '')"
      />
      <USelect
        class="bio-list__select"
        :model-value="statusFilter"
        :items="STATUS_ITEMS"
        value-key="value"
        @update:model-value="statusFilter = $event as BioStatus | 'all'"
      />
      <USelect
        v-if="isAdmin"
        class="bio-list__select"
        :model-value="accountFilter"
        :items="accountItems"
        value-key="value"
        @update:model-value="accountFilter = String($event ?? 'all')"
      />
    </div>

    <p v-if="store.listError" class="bio-list__error">{{ store.listError }}</p>

    <div v-if="store.listPending" class="bio-list__state">Carregando bios...</div>

    <div v-else-if="!filteredBios.length" class="bio-list__empty">
      <UIcon name="i-lucide-link" class="bio-list__empty-icon" />
      <p class="bio-list__empty-title">Nenhuma bio por aqui ainda</p>
      <p class="bio-list__empty-hint">
        Crie a primeira bio para comecar a montar a pagina de link-in-bio do cliente.
      </p>
      <UButton
        icon="i-lucide-plus"
        color="primary"
        variant="soft"
        label="Nova bio"
        @click="createOpen = true"
      />
    </div>

    <div v-else class="bio-list__table-wrap">
      <table class="bio-list__table">
        <thead>
          <tr>
            <th>Nome</th>
            <th>Slug</th>
            <th v-if="isAdmin">Cliente</th>
            <th>Status</th>
            <th>Atualizado</th>
            <th class="bio-list__th-actions">Acoes</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="bio in filteredBios"
            :key="bio.id"
            class="bio-list__row"
            @click="openEditor(bio)"
          >
            <td class="bio-list__cell-name">{{ bio.name }}</td>
            <td class="bio-list__cell-slug">/{{ bio.slug }}</td>
            <td v-if="isAdmin">{{ bio.accountName || '—' }}</td>
            <td>
              <span
                class="bio-list__badge"
                :class="
                  bio.status === 'published'
                    ? 'bio-list__badge--published'
                    : 'bio-list__badge--draft'
                "
              >
                {{ statusLabel(bio.status) }}
              </span>
            </td>
            <td>{{ formatDate(bio.updatedAt) }}</td>
            <td class="bio-list__cell-actions" @click.stop>
              <UButton
                icon="i-lucide-pencil"
                color="neutral"
                variant="ghost"
                size="sm"
                aria-label="Editar bio"
                @click="openEditor(bio)"
              />
              <UButton
                icon="i-lucide-copy"
                color="neutral"
                variant="ghost"
                size="sm"
                aria-label="Duplicar bio"
                :loading="duplicatingId === bio.id"
                @click="onDuplicate(bio)"
              />
              <UButton
                v-if="bioPublicUrl(bio)"
                icon="i-lucide-external-link"
                color="neutral"
                variant="ghost"
                size="sm"
                aria-label="Ver online"
                :to="bioPublicUrl(bio)"
                target="_blank"
                rel="noopener"
              />
              <UButton
                icon="i-lucide-trash-2"
                color="error"
                variant="ghost"
                size="sm"
                aria-label="Excluir bio"
                :loading="deletingId === bio.id"
                @click="onDelete(bio)"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <BioCreateModal
      v-model:open="createOpen"
      :creating="creating"
      :is-admin="isAdmin"
      :tenants="tenantOptions"
      @submit="onCreate"
    />
    <p v-if="createError" class="bio-list__error">{{ createError }}</p>
  </section>
</template>

<style scoped>
.bio-list {
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
  padding: 1.5rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.bio-list__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.bio-list__title {
  font-size: 1.4rem;
  font-weight: 700;
  color: var(--text-main);
}

.bio-list__subtitle {
  margin: 0.2rem 0 0;
  font-size: 0.88rem;
  color: var(--text-muted);
}

.bio-list__filters {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.bio-list__search {
  flex: 1;
  min-width: 220px;
}

.bio-list__select {
  min-width: 180px;
}

.bio-list__error {
  margin: 0;
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.16);
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-soft);
  font-size: 0.88rem;
}

.bio-list__state {
  color: var(--text-muted);
  font-size: 0.9rem;
  padding: 1.5rem 0;
}

.bio-list__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  padding: 3rem 1.5rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface-2) / 0.4);
  text-align: center;
}

.bio-list__empty-icon {
  width: 2.2rem;
  height: 2.2rem;
  color: var(--text-muted);
}

.bio-list__empty-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: var(--text-main);
}

.bio-list__empty-hint {
  margin: 0;
  max-width: 360px;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.bio-list__table-wrap {
  overflow-x: auto;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
}

.bio-list__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.88rem;
}

.bio-list__table thead th {
  text-align: left;
  padding: 0.75rem 1rem;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
  border-bottom: 1px solid var(--line-soft);
}

.bio-list__th-actions {
  text-align: right;
}

.bio-list__row {
  cursor: pointer;
  transition: background 0.12s ease;
}

.bio-list__row:hover {
  background: rgb(var(--surface-2) / 0.5);
}

.bio-list__table td {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--line-soft);
  color: var(--text-main);
}

.bio-list__row:last-child td {
  border-bottom: none;
}

.bio-list__cell-name {
  font-weight: 600;
}

.bio-list__cell-slug {
  color: var(--text-muted);
  font-family: ui-monospace, monospace;
  font-size: 0.82rem;
}

.bio-list__badge {
  display: inline-flex;
  align-items: center;
  padding: 0.15rem 0.6rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 700;
}

.bio-list__badge--published {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.bio-list__badge--draft {
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
}

.bio-list__cell-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.25rem;
}
</style>
