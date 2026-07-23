<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useCoreAccountStore } from '../../../layers/core/stores/account'
import type {
  SocialPublishingPost,
  SocialPublishingPostInput,
} from '~/domain/social-publishing/model'
import { useAuthStore } from '~/stores/auth'
import { useSocialPublishingStore } from '~/stores/social-publishing'
import { useUiStore } from '~/stores/ui'

type WorkspaceTab = 'queue' | 'content' | 'analytics' | 'connection'

interface WorkspaceTabOption {
  id: WorkspaceTab
  label: string
  icon: string
}

const auth = useAuthStore()
const accountStore = useCoreAccountStore()
const store = useSocialPublishingStore()
const ui = useUiStore()
const {
  connection,
  posts,
  overview,
  initialized,
  loading,
  error,
  savingPost,
  connectionBusy,
  analyticsSyncing,
  busyPostIds,
  scheduledPosts,
  contentPosts,
} = storeToRefs(store)

const activeTab = ref<WorkspaceTab>('queue')
const composerOpen = ref(false)
const selectedPost = ref<SocialPublishingPost | null>(null)
const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
const permissionKeys = computed(() => auth.effectivePermissionKeys)
const canView = computed(
  () => isPlatformAdmin.value || permissionKeys.value.includes('social_publishing.view'),
)
const canManage = computed(
  () => isPlatformAdmin.value || permissionKeys.value.includes('social_publishing.manage'),
)
const canConnect = computed(
  () => isPlatformAdmin.value || permissionKeys.value.includes('social_publishing.connect'),
)
const canAnalytics = computed(
  () => isPlatformAdmin.value || permissionKeys.value.includes('social_publishing.analytics'),
)

const tabs = computed<WorkspaceTabOption[]>(() => {
  const options: WorkspaceTabOption[] = [
    { id: 'queue', label: 'Fila', icon: 'i-lucide-calendar-clock' },
    { id: 'content', label: 'Conteúdo', icon: 'i-lucide-layout-grid' },
  ]
  if (canAnalytics.value) {
    options.push({ id: 'analytics', label: 'Analytics', icon: 'i-lucide-chart-no-axes-column' })
  }
  options.push({ id: 'connection', label: 'Conexão', icon: 'i-lucide-plug' })
  return options
})

const activeTabLabel = computed(
  () => tabs.value.find((tab) => tab.id === activeTab.value)?.label || 'Postagens',
)
function isConfirmed(value: unknown): boolean {
  return Boolean(value && typeof value === 'object' && 'confirmed' in value && value.confirmed)
}

function openComposer(post: SocialPublishingPost | null = null): void {
  if (!canManage.value) return
  store.clearError()
  selectedPost.value = post
  composerOpen.value = true
}

function closeComposer(): void {
  composerOpen.value = false
  selectedPost.value = null
}

async function savePost(input: SocialPublishingPostInput): Promise<void> {
  const result = await store.savePost(input, selectedPost.value?.id)
  if (!result) return
  closeComposer()
  ui.success('Rascunho salvo.')
}

async function schedulePost(input: SocialPublishingPostInput): Promise<void> {
  const result = await store.saveAndSchedule(input, selectedPost.value?.id)
  if (!result) return
  closeComposer()
  activeTab.value = 'queue'
  ui.success('Publicação agendada.')
}

async function cancelPost(post: SocialPublishingPost): Promise<void> {
  const answer = await ui.confirm({
    title: 'Cancelar agendamento?',
    message: 'A publicação deixará a fila e não será enviada no horário programado.',
    confirmLabel: 'Cancelar agendamento',
  })
  if (!isConfirmed(answer)) return
  if (await store.cancel(post)) ui.success('Agendamento cancelado.')
}

async function retryPost(post: SocialPublishingPost): Promise<void> {
  if (await store.retry(post)) ui.success('Nova tentativa solicitada.')
}

async function connect(accessToken: string): Promise<void> {
  if (await store.connect(accessToken)) ui.success('Instagram conectado.')
}

async function disconnect(): Promise<void> {
  const answer = await ui.confirm({
    title: 'Desconectar Instagram?',
    message: 'Novas publicações não poderão ser enviadas até uma nova conexão.',
    confirmLabel: 'Desconectar',
  })
  if (!isConfirmed(answer)) return
  if (await store.disconnect()) ui.success('Instagram desconectado.')
}

async function syncAnalytics(): Promise<void> {
  if (await store.refreshAnalytics()) ui.success('Sincronização de analytics enfileirada.')
}

function reload(): void {
  void store.initialize({ includeAnalytics: canAnalytics.value })
}

watch(
  () => [canView.value, canAnalytics.value] as const,
  ([allowed, includeAnalytics]) => {
    if (allowed) void store.initialize({ includeAnalytics })
  },
  { immediate: true },
)

watch(
  () => accountStore.activeAccountId,
  () => closeComposer(),
)

watch(tabs, (options) => {
  if (!options.some((tab) => tab.id === activeTab.value)) activeTab.value = 'queue'
})
</script>

<template>
  <main class="sp-workspace">
    <div class="sp-workspace__top">
      <AdminPageHeader
        eyebrow="Instagram"
        title="Agendamento de postagens"
        description="Prepare, agende e acompanhe publicações do cliente em um só lugar."
      />
      <UButton
        v-if="canView && canManage"
        type="button"
        color="primary"
        icon="i-lucide-plus"
        label="Nova publicação"
        @click="openComposer()"
      />
    </div>

    <section v-if="!canView" class="sp-workspace__state omni-glass" aria-labelledby="sp-denied">
      <UIcon name="i-lucide-shield-x" aria-hidden="true" />
      <h2 id="sp-denied">Acesso não autorizado</h2>
      <p>
        Seu perfil não possui a permissão
        <code>social_publishing.view</code>
        .
      </p>
    </section>

    <template v-else>
      <div v-if="error && !composerOpen" class="sp-workspace__error" role="alert">
        <UIcon name="i-lucide-circle-alert" aria-hidden="true" />
        <span>{{ error }}</span>
        <UButton
          type="button"
          color="error"
          variant="ghost"
          size="sm"
          label="Tentar novamente"
          @click="reload"
        />
      </div>

      <div v-if="loading && !initialized" class="sp-workspace__loading" aria-live="polite">
        <span class="sp-workspace__spinner" aria-hidden="true"></span>
        <p>Carregando postagens deste cliente…</p>
      </div>

      <template v-else>
        <SocialPublishingSummaryCards :posts="posts" :connection="connection" />

        <div v-if="!connection?.connected" class="sp-workspace__connection-note">
          <div>
            <UIcon name="i-lucide-unplug" aria-hidden="true" />
            <span>
              O Instagram está desconectado. Rascunhos podem ser preparados, mas o envio depende de
              uma conexão ativa.
            </span>
          </div>
          <UButton
            type="button"
            color="neutral"
            variant="soft"
            size="sm"
            label="Ver conexão"
            @click="activeTab = 'connection'"
          />
        </div>

        <nav class="sp-tabs" aria-label="Áreas do módulo" role="tablist">
          <button
            v-for="tab in tabs"
            :id="`sp-tab-${tab.id}`"
            :key="tab.id"
            type="button"
            role="tab"
            class="sp-tabs__item"
            :class="{ 'sp-tabs__item--active': activeTab === tab.id }"
            :aria-selected="activeTab === tab.id"
            :aria-controls="`sp-panel-${tab.id}`"
            @click="activeTab = tab.id"
          >
            <UIcon :name="tab.icon" aria-hidden="true" />
            <span>{{ tab.label }}</span>
          </button>
        </nav>

        <section
          :id="`sp-panel-${activeTab}`"
          class="sp-workspace__panel"
          role="tabpanel"
          :aria-labelledby="`sp-tab-${activeTab}`"
          :aria-label="activeTabLabel"
          tabindex="0"
        >
          <SocialPublishingPostList
            v-if="activeTab === 'queue'"
            :posts="scheduledPosts"
            mode="queue"
            :can-manage="canManage"
            :busy-post-ids="busyPostIds"
            @edit="openComposer"
            @cancel="cancelPost"
            @retry="retryPost"
          />
          <SocialPublishingPostList
            v-else-if="activeTab === 'content'"
            :posts="contentPosts"
            mode="content"
            :can-manage="canManage"
            :busy-post-ids="busyPostIds"
            @edit="openComposer"
            @cancel="cancelPost"
            @retry="retryPost"
          />
          <SocialPublishingAnalytics
            v-else-if="activeTab === 'analytics' && canAnalytics"
            :overview="overview"
            :posts="posts"
            :syncing="analyticsSyncing"
            :can-sync="canAnalytics"
            @sync="syncAnalytics"
          />
          <SocialPublishingConnectionCard
            v-else-if="activeTab === 'connection'"
            :connection="connection"
            :busy="connectionBusy"
            :can-connect="canConnect"
            @connect="connect"
            @disconnect="disconnect"
          />
        </section>
      </template>
    </template>

    <SocialPublishingComposerDrawer
      v-if="canManage"
      v-model="composerOpen"
      :post="selectedPost"
      :busy="savingPost"
      :error="error"
      @save="savePost"
      @schedule="schedulePost"
    />
  </main>
</template>

<style scoped>
.sp-workspace {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  flex-direction: column;
  gap: 1rem;
  padding: 1.35rem;
}
.sp-workspace__top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
.sp-workspace__state,
.sp-workspace__loading {
  display: grid;
  min-height: 18rem;
  place-content: center;
  justify-items: center;
  padding: 2rem;
  color: rgb(var(--muted));
  text-align: center;
}
.sp-workspace__state {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
}
.sp-workspace__state :deep(svg) {
  width: 2rem;
  height: 2rem;
  color: rgb(var(--danger));
}
.sp-workspace__state h2 {
  margin: 0.8rem 0 0;
  color: rgb(var(--text));
  font-size: 1rem;
}
.sp-workspace__state p {
  margin: 0.3rem 0 0;
  font-size: 0.82rem;
}
.sp-workspace__error,
.sp-workspace__connection-note,
.sp-workspace__connection-note > div {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}
.sp-workspace__error {
  padding: 0.65rem 0.75rem;
  border-radius: var(--radius-xs);
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.1);
  font-size: 0.8rem;
}
.sp-workspace__error span {
  flex: 1;
}
.sp-workspace__connection-note {
  justify-content: space-between;
  padding: 0.65rem 0.8rem;
  border: 1px solid rgb(var(--warning) / 0.3);
  border-radius: var(--radius-xs);
  color: rgb(var(--text));
  background: rgb(var(--warning) / 0.1);
  font-size: 0.78rem;
}
.sp-workspace__spinner {
  width: 1.8rem;
  height: 1.8rem;
  border: 2px solid rgb(var(--border));
  border-top-color: rgb(var(--primary));
  border-radius: 999px;
  animation: sp-spin 0.8s linear infinite;
}
.sp-workspace__loading p {
  margin: 0.7rem 0 0;
  font-size: 0.82rem;
}
.sp-tabs {
  display: flex;
  gap: 0.25rem;
  padding: 0.25rem;
  overflow-x: auto;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.68);
}
.sp-tabs__item {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  min-height: 2.4rem;
  padding: 0 0.8rem;
  border: 0;
  border-radius: var(--radius-xs);
  color: rgb(var(--muted));
  background: transparent;
  cursor: pointer;
  font: inherit;
  font-size: 0.8rem;
  font-weight: 650;
  white-space: nowrap;
}
.sp-tabs__item:hover {
  color: rgb(var(--text));
  background: rgb(var(--surface) / 0.7);
}
.sp-tabs__item--active {
  color: rgb(var(--primary));
  background: rgb(var(--surface));
  box-shadow: var(--shadow-xs);
}
.sp-tabs__item:focus-visible,
.sp-workspace__panel:focus-visible {
  outline: 2px solid rgb(var(--ring));
  outline-offset: 2px;
}
.sp-workspace__panel {
  min-width: 0;
  outline: none;
}
@keyframes sp-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .sp-workspace__spinner {
    animation: none;
  }
}
@media (max-width: 640px) {
  .sp-workspace {
    padding: 1rem;
  }
  .sp-workspace__top,
  .sp-workspace__connection-note {
    align-items: stretch;
    flex-direction: column;
  }
  .sp-tabs__item {
    flex: 1 0 auto;
  }
}
</style>
