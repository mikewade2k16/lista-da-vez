<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import BioLivePreview from '~/components/bio/BioLivePreview.vue'
import BioPublishBar from '~/components/bio/BioPublishBar.vue'
import BioSectionBranding from '~/components/bio/sections/BioSectionBranding.vue'
import BioSectionLinks from '~/components/bio/sections/BioSectionLinks.vue'
import BioSectionMeta from '~/components/bio/sections/BioSectionMeta.vue'
import BioSectionSlides from '~/components/bio/sections/BioSectionSlides.vue'
import BioSectionStores from '~/components/bio/sections/BioSectionStores.vue'
import BioSectionVideo from '~/components/bio/sections/BioSectionVideo.vue'
import { useBioEditor } from '~/composables/useBioEditor'
import { useBioStore } from '~/stores/bio'
import { useTenantsStore } from '~/stores/tenants'
import { useRuntimeConfig } from '#app'
import { useCoreAccountStore } from '../../../layers/core/stores/account'
import type { BioData, BioMediaKind } from '~/domain/bio/types'

// Shell do editor de uma bio: faixa de status/publicar no topo (BioPublishBar)
// + seletor de cliente (so admin) + sidebar de secoes + painel da secao ativa +
// preview ao vivo. Inspirado no AutomationWorkspace (status bar + nav lateral).
// O rascunho salva sozinho (auto-save no useBioEditor); o usuario so decide
// quando publicar. Ctrl+Z desfaz a ultima mudanca estavel (pilha de undo).

const props = defineProps<{ bioId: string }>()

const store = useBioStore()
const tenants = useTenantsStore()
const editor = useBioEditor()
const coreAccount = useCoreAccountStore()

type SectionId = 'meta' | 'branding' | 'video' | 'links' | 'slides' | 'stores'
interface SectionDef {
  id: SectionId
  label: string
  icon: string
}

const SECTIONS: SectionDef[] = [
  { id: 'meta', label: 'Meta', icon: 'i-lucide-tag' },
  { id: 'branding', label: 'Branding', icon: 'i-lucide-image' },
  { id: 'video', label: 'Video e layout', icon: 'i-lucide-clapperboard' },
  { id: 'links', label: 'Links e menu', icon: 'i-lucide-link' },
  { id: 'slides', label: 'Slides do topo', icon: 'i-lucide-gallery-horizontal' },
  { id: 'stores', label: 'Lojas e lightbox', icon: 'i-lucide-map-pin' },
]

const active = ref<SectionId>('meta')
const publishing = ref(false)
const actionError = ref('')
const showPreview = ref(true)
// Fonte do preview: 'draft' (rascunho em edicao) ou 'published' (o que esta no
// ar). O switch na BioPublishBar troca este valor para comparar as versoes.
const previewSource = ref<'draft' | 'published'>('draft')
// Cliente selecionado no editor (so admin). Inicia com a account da bio ativa.
const movingAccount = ref(false)

const status = computed(() => store.activeBio?.status ?? 'draft')
const isAdmin = computed(() => store.isAdmin)

const publishedData = computed<BioData | null>(() => store.activeBio?.dataPublished ?? null)

const tenantItems = computed(() => [
  { label: 'Sem cliente (agencia)', value: 'agency' },
  ...tenants.tenants.map((tenant: { id: string; name: string }) => ({
    label: tenant.name,
    value: tenant.id,
  })),
])

// Valor do select de cliente: o accountId da bio, ou o sentinela 'agency' se
// a account nao estiver na lista de clientes (caso da propria agencia).
const accountValue = computed(() => {
  const accountId = store.activeBio?.accountId ?? ''
  const known = tenants.tenants.some((tenant: { id: string }) => tenant.id === accountId)
  return known ? accountId : 'agency'
})

const accountNotice = ref('')
let accountNoticeTimer: ReturnType<typeof setTimeout> | null = null

async function onChangeAccount(next: string) {
  // "Sem cliente (agencia)" = move para a account ativa do admin (a agencia);
  // outro valor = a account daquele cliente.
  const target =
    next === 'agency' ? String(coreAccount.activeAccountId || '').trim() : String(next || '').trim()
  if (!target || target === store.activeBio?.accountId) {
    return
  }
  movingAccount.value = true
  actionError.value = ''
  try {
    const result = await store.moveBioAccount(props.bioId, target)
    if (!result.ok) {
      actionError.value = result.message
      return
    }
    // Feedback visivel: o select ja reflete o novo accountId (activeBio
    // atualizado), mas confirmamos a troca para nao parecer que "nada mudou".
    accountNotice.value = 'Cliente atualizado'
    if (accountNoticeTimer) {
      clearTimeout(accountNoticeTimer)
    }
    accountNoticeTimer = setTimeout(() => {
      accountNotice.value = ''
    }, 2500)
  } finally {
    movingAccount.value = false
  }
}

// Ctrl+Z / Cmd+Z desfaz a ultima mudanca estavel. Ignora quando o foco esta num
// input de texto (deixa o undo nativo do campo funcionar).
function onKeydown(event: KeyboardEvent) {
  if (!(event.ctrlKey || event.metaKey) || event.shiftKey || event.key.toLowerCase() !== 'z') {
    return
  }
  const target = event.target as HTMLElement | null
  const tag = target?.tagName?.toLowerCase()
  if (tag === 'input' || tag === 'textarea' || target?.isContentEditable) {
    return
  }
  event.preventDefault()
  editor.undo()
}

const config = useRuntimeConfig()
const bioFrontUrl = String((config.public as Record<string, unknown>).bioFrontUrl || '').replace(
  /\/$/,
  '',
)

// Purge do cache do front bio apos publicar: a versao publica reflete na hora
// (em prod o SWR de 300s seria o atraso). Best-effort: falha nao bloqueia.
async function purgeFront() {
  if (!bioFrontUrl || !editor.slug.value) {
    return
  }
  try {
    await $fetch(`${bioFrontUrl}/api/bio/purge`, {
      method: 'POST',
      body: { slug: editor.slug.value },
    })
  } catch {
    // cache purge e best-effort; o SWR revalida sozinho no pior caso.
  }
}

function uploadMedia(kind: BioMediaKind, file: File) {
  return editor.uploadMedia(props.bioId, kind, file)
}

function onDraftUpdate(next: BioData) {
  editor.draft.value = next
}

async function onPublish() {
  // Garante que o auto-save pendente persistiu antes de publicar.
  if (editor.dirty.value) {
    const saved = await editor.flushSave(props.bioId)
    if (!saved.ok) {
      actionError.value = saved.message
      return
    }
  }
  publishing.value = true
  actionError.value = ''
  try {
    const result = await store.publishBio(props.bioId)
    if (!result.ok) {
      actionError.value = result.message
      return
    }
    // Re-hidrata para o snapshot publicado virar igual ao draft (zera o aviso
    // de "alteracoes nao publicadas").
    await editor.load(props.bioId)
    // Invalida o cache do front para a bio publica refletir na hora.
    await purgeFront()
  } finally {
    publishing.value = false
  }
}

async function onUnpublish() {
  publishing.value = true
  actionError.value = ''
  try {
    const result = await store.unpublishBio(props.bioId)
    if (!result.ok) {
      actionError.value = result.message
    }
  } finally {
    publishing.value = false
  }
}

onMounted(() => {
  void editor.load(props.bioId)
  if (isAdmin.value) {
    void tenants.ensureLoaded()
  }
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  if (accountNoticeTimer) {
    clearTimeout(accountNoticeTimer)
  }
})
</script>

<template>
  <section class="bio-editor">
    <header class="bio-editor__topbar">
      <div class="bio-editor__topbar-main">
        <NuxtLink
          to="/site/bio"
          class="bio-editor__back"
          title="Voltar para a lista"
          aria-label="Voltar para a lista"
        >
          <UIcon name="i-lucide-arrow-left" />
        </NuxtLink>
        <h1 class="bio-editor__title">{{ editor.name.value || 'Editar bio' }}</h1>
        <span v-if="editor.slug.value" class="bio-editor__slug">/{{ editor.slug.value }}</span>
        <USelect
          v-if="isAdmin"
          class="bio-editor__account-select"
          :model-value="accountValue"
          :items="tenantItems"
          value-key="value"
          size="xs"
          icon="i-lucide-building-2"
          title="Cliente (account) da bio"
          :loading="movingAccount"
          :disabled="movingAccount"
          @update:model-value="onChangeAccount(String($event ?? 'agency'))"
        />
        <span v-if="accountNotice" class="bio-editor__account-notice">{{ accountNotice }}</span>
      </div>

      <div class="bio-editor__topbar-actions">
        <UButton
          :icon="showPreview ? 'i-lucide-eye' : 'i-lucide-eye-off'"
          color="neutral"
          variant="ghost"
          size="sm"
          :title="showPreview ? 'Previa ao vivo: ligada' : 'Previa ao vivo: desligada'"
          aria-label="Alternar previa ao vivo"
          @click="showPreview = !showPreview"
        />
        <BioPublishBar
          :status="status"
          :slug="editor.slug.value"
          :dirty="editor.dirty.value"
          :unpublished-changes="editor.hasUnpublishedChanges.value"
          :saving="editor.saving.value"
          :save-state="editor.saveState.value"
          :publishing="publishing"
          :can-undo="editor.canUndo.value"
          @publish="onPublish"
          @unpublish="onUnpublish"
          @undo="editor.undo"
        />
      </div>
    </header>

    <p v-if="actionError || editor.errorMessage.value" class="bio-editor__error">
      {{ actionError || editor.errorMessage.value }}
    </p>

    <div v-if="editor.loading.value" class="bio-editor__loading">Carregando bio...</div>

    <div v-else class="bio-editor__body" :class="{ 'bio-editor__body--with-preview': showPreview }">
      <nav class="bio-editor__nav" aria-label="Secoes da bio">
        <p class="bio-editor__nav-head">Conteudo</p>
        <button
          v-for="section in SECTIONS"
          :key="section.id"
          type="button"
          class="bio-editor__nav-item"
          :class="{ 'bio-editor__nav-item--active': active === section.id }"
          @click="active = section.id"
        >
          <UIcon :name="section.icon" class="bio-editor__nav-icon" aria-hidden="true" />
          <span class="bio-editor__nav-label">{{ section.label }}</span>
        </button>
      </nav>

      <div class="bio-editor__panel">
        <BioSectionMeta
          v-if="active === 'meta'"
          :draft="editor.draft.value"
          @update:draft="onDraftUpdate"
        />
        <BioSectionBranding
          v-else-if="active === 'branding'"
          :draft="editor.draft.value"
          :bio-id="bioId"
          :upload-media="uploadMedia"
          :is-uploading="editor.isUploading"
          @update:draft="onDraftUpdate"
        />
        <BioSectionVideo
          v-else-if="active === 'video'"
          :draft="editor.draft.value"
          :bio-id="bioId"
          :upload-media="uploadMedia"
          :is-uploading="editor.isUploading"
          @update:draft="onDraftUpdate"
        />
        <BioSectionLinks
          v-else-if="active === 'links'"
          :draft="editor.draft.value"
          @update:draft="onDraftUpdate"
        />
        <BioSectionSlides
          v-else-if="active === 'slides'"
          :draft="editor.draft.value"
          :bio-id="bioId"
          :upload-media="uploadMedia"
          :is-uploading="editor.isUploading"
          @update:draft="onDraftUpdate"
        />
        <BioSectionStores
          v-else-if="active === 'stores'"
          :draft="editor.draft.value"
          :bio-id="bioId"
          :upload-media="uploadMedia"
          :is-uploading="editor.isUploading"
          @update:draft="onDraftUpdate"
        />
      </div>

      <aside v-if="showPreview" class="bio-editor__preview">
        <BioLivePreview
          :draft="editor.draft.value"
          :published="publishedData"
          :source="previewSource"
          :is-published="status === 'published'"
          @update:source="previewSource = $event"
        />
      </aside>
    </div>
  </section>
</template>

<style scoped>
.bio-editor {
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
  padding: 1.5rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

/* Topbar unica: voltar + titulo + slug + cliente (esquerda) | previa + acoes
   (direita). Tudo numa linha (faz wrap so em telas estreitas) — sem as 4 linhas
   separadas que desperdicavam espaco vertical. */
.bio-editor__topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.6rem 1rem;
  padding: 0.45rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.8);
  box-shadow: var(--shadow-card);
}

.bio-editor__topbar-main {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
  min-width: 0;
}

.bio-editor__topbar-actions {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
}

.bio-editor__back {
  display: inline-flex;
  align-items: center;
  color: var(--text-muted);
  text-decoration: none;
}

.bio-editor__back:hover {
  color: rgb(var(--primary));
}

.bio-editor__title {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
}

.bio-editor__slug {
  font-size: 0.8rem;
  color: var(--text-muted);
  font-family: ui-monospace, monospace;
}

.bio-editor__error {
  margin: 0;
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.16);
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-soft);
  font-size: 0.9rem;
}

.bio-editor__account-select {
  min-width: 220px;
}

/* Itens do dropdown de cliente: texto menor e sem truncar, p/ caber o nome
   completo do cliente (nomes longos vinham cortados). */
.bio-editor__account-select :deep([role='option']) {
  font-size: 0.78rem;
  white-space: normal;
}

.bio-editor__account-notice {
  font-size: 0.78rem;
  color: rgb(var(--success));
  font-weight: 600;
}

.bio-editor__loading {
  color: var(--text-muted);
  font-size: 0.9rem;
  padding: 1.5rem;
}

.bio-editor__body {
  display: grid;
  grid-template-columns: 232px minmax(0, 1fr);
  gap: 1.5rem;
  align-items: start;
  flex: 1;
  min-height: 0;
}

.bio-editor__body--with-preview {
  grid-template-columns: 232px minmax(0, 1fr) 380px;
}

.bio-editor__preview {
  min-width: 0;
}

@media (max-width: 1180px) {
  .bio-editor__body--with-preview {
    grid-template-columns: 232px minmax(0, 1fr);
  }

  .bio-editor__preview {
    display: none;
  }
}

.bio-editor__nav {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  position: sticky;
  top: 0;
}

.bio-editor__nav-head {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
  padding: 0 0.85rem;
  margin-bottom: 0.5rem;
}

.bio-editor__nav-item {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  width: 100%;
  padding: 0.7rem 0.85rem;
  border-radius: 0.7rem;
  border: none;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.92rem;
  font-weight: 500;
  text-align: left;
  transition:
    background 0.12s ease,
    color 0.12s ease;
}

.bio-editor__nav-item:hover {
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
}

.bio-editor__nav-item--active {
  background: rgb(var(--primary) / 0.15);
  color: var(--text-main);
  font-weight: 600;
}

.bio-editor__nav-item--active .bio-editor__nav-icon {
  color: rgb(var(--primary));
}

.bio-editor__nav-icon {
  width: 1.1rem;
  height: 1.1rem;
  flex-shrink: 0;
}

.bio-editor__nav-label {
  flex: 1;
  min-width: 0;
}

.bio-editor__panel {
  min-width: 0;
}

@media (max-width: 880px) {
  .bio-editor__body {
    grid-template-columns: 1fr;
  }

  .bio-editor__nav {
    position: static;
    flex-direction: row;
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .bio-editor__nav-head {
    display: none;
  }

  .bio-editor__nav-item {
    width: auto;
    white-space: nowrap;
  }
}
</style>
