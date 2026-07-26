<script setup lang="ts">
import { storeToRefs } from 'pinia'

import { useCoreAccountStore } from '../../../layers/core/stores/account'
import type {
  SocialPublishingPost,
  SocialPublishingPostInput,
} from '~/domain/social-publishing/model'
import { useAuthStore } from '~/stores/auth'
import { useSocialPublishingPortfolioStore } from '~/stores/social-publishing-portfolio'
import { SOCIAL_PUBLISHING_PAGE_SIZE, useSocialPublishingStore } from '~/stores/social-publishing'
import { useUiStore } from '~/stores/ui'
import SocialPublishingPortfolioView from './SocialPublishingPortfolio.vue'
import SocialPublishingWorkspaceHeader from './SocialPublishingWorkspaceHeader.vue'
import { useSocialPublishingPolling } from './useSocialPublishingPolling'

type WorkspaceTab = 'queue' | 'content' | 'analytics' | 'connection'

interface WorkspaceTabOption {
  id: WorkspaceTab
  label: string
  icon: string
}

const auth = useAuthStore()
const accountStore = useCoreAccountStore()
const store = useSocialPublishingStore()
const portfolioStore = useSocialPublishingPortfolioStore()
const ui = useUiStore()
const {
  connection,
  posts,
  overview,
  initialized,
  loading,
  refreshing,
  queueLoading,
  contentLoading,
  error,
  savingPost,
  connectionBusy,
  analyticsSyncing,
  analyticsSyncPending,
  busyPostIds,
  queuePage,
  contentPage,
  queueHasNext,
  contentHasNext,
  scheduledPosts,
  contentPosts,
  hasPollingWork,
  nextPollingWakeAt,
} = storeToRefs(store)
const {
  scope,
  portfolio,
  selectedClientId,
  scopeResolved,
  loadingScope,
  loadingPortfolio,
  switching,
  error: portfolioError,
  scopeHostId,
  portfolioMode,
} = storeToRefs(portfolioStore)

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
const individualMode = computed(
  () =>
    scopeResolved.value &&
    Boolean(scope.value) &&
    (!scope.value?.canSelect || Boolean(selectedClientId.value)),
)
const clientOptions = computed(() => [
  { value: '', label: 'Todos os clientes' },
  ...(scope.value?.clients || []).map((client) => ({
    value: client.id,
    label: client.name,
  })),
])
const scopeError = computed(() => (scopeResolved.value && !scope.value ? portfolioError.value : ''))
const pollingEnabled = computed(
  () => canView.value && individualMode.value && !switching.value && hasPollingWork.value,
)
const pollingWakeAt = computed(() =>
  canView.value && individualMode.value && !switching.value ? nextPollingWakeAt.value : null,
)

useSocialPublishingPolling({
  enabled: pollingEnabled,
  wakeAt: pollingWakeAt,
  poll: store.poll,
})

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

let contextVersion = 0
let internalAccountSwitch = false

function isConfirmed(value: unknown): boolean {
  return Boolean(value && typeof value === 'object' && 'confirmed' in value && value.confirmed)
}

function openComposer(post: SocialPublishingPost | null = null): void {
  if (!canManage.value || !individualMode.value) return
  store.clearError()
  selectedPost.value = post
  composerOpen.value = true
}

function closeComposer(): void {
  composerOpen.value = false
  selectedPost.value = null
}

function reconcileWorkspace(): void {
  if (individualMode.value) void store.refreshWorkspace()
}

async function savePost(input: SocialPublishingPostInput): Promise<void> {
  if (!individualMode.value) return
  const result = await store.savePost(input, selectedPost.value?.id)
  if (!result) return
  closeComposer()
  reconcileWorkspace()
  ui.success('Rascunho salvo.')
}

async function schedulePost(input: SocialPublishingPostInput): Promise<void> {
  if (!individualMode.value) return
  const result = await store.saveAndSchedule(input, selectedPost.value?.id)
  if (!result) return
  closeComposer()
  activeTab.value = 'queue'
  reconcileWorkspace()
  ui.success('Publicação agendada.')
}

async function cancelPost(post: SocialPublishingPost): Promise<void> {
  if (!individualMode.value) return
  const answer = await ui.confirm({
    title: 'Cancelar agendamento?',
    message: 'A publicação deixará a fila e não será enviada no horário programado.',
    confirmLabel: 'Cancelar agendamento',
  })
  if (!isConfirmed(answer)) return
  if (await store.cancel(post)) {
    reconcileWorkspace()
    ui.success('Agendamento cancelado.')
  }
}

async function retryPost(post: SocialPublishingPost): Promise<void> {
  if (!individualMode.value) return
  if (await store.retry(post)) {
    reconcileWorkspace()
    ui.success('Nova tentativa solicitada.')
  }
}

async function connect(accessToken: string): Promise<void> {
  if (!individualMode.value) return
  if (await store.connect(accessToken)) ui.success('Instagram conectado.')
}

async function disconnect(): Promise<void> {
  if (!individualMode.value) return
  const answer = await ui.confirm({
    title: 'Desconectar Instagram?',
    message: 'Novas publicações não poderão ser enviadas até uma nova conexão.',
    confirmLabel: 'Desconectar',
  })
  if (!isConfirmed(answer)) return
  if (await store.disconnect()) ui.success('Instagram desconectado.')
}

async function syncAnalytics(): Promise<void> {
  if (!individualMode.value) return
  const queued = await store.refreshAnalytics()
  if (queued === null) return
  if (queued > 0) ui.success('Sincronização de analytics enfileirada.')
  else ui.info('Não há publicações aguardando sincronização.', 'Analytics')
}

async function initializePublishingContext(): Promise<void> {
  const version = ++contextVersion
  closeComposer()
  store.setPortfolioMode(true)
  store.suspend()
  portfolioStore.reset()

  if (!canView.value || !accountStore.activeAccountId) return
  const nextScope = await portfolioStore.loadScope()
  if (version !== contextVersion || !nextScope) return

  if (portfolioStore.portfolioMode) {
    if (canAnalytics.value) await portfolioStore.loadPortfolio()
    return
  }
  store.setPortfolioMode(false)
  await store.initialize({ includeAnalytics: canAnalytics.value })
}

async function selectPublishingClient(clientId: string): Promise<void> {
  const normalized = String(clientId || '').trim()
  const previousClientId = selectedClientId.value
  if (!portfolioStore.selectClient(normalized)) return

  const version = ++contextVersion
  closeComposer()
  store.setPortfolioMode(true)
  store.suspend()
  portfolioStore.prepareAccountSwitch()
  portfolioStore.setSwitching(true)

  const targetAccountId = normalized || scopeHostId.value
  const targetExists = accountStore.accounts.some((account) => account.id === targetAccountId)
  if (!targetAccountId || !targetExists) {
    portfolioStore.selectClient(previousClientId)
    portfolioStore.setError('Não foi possível localizar a conta selecionada.')
    portfolioStore.setSwitching(false)
    return
  }

  try {
    internalAccountSwitch = true
    if (targetAccountId !== accountStore.activeAccountId) {
      await accountStore.switchAccount(targetAccountId)
    }
    if (version !== contextVersion) return

    if (normalized) {
      store.setPortfolioMode(false)
      await store.initialize({ includeAnalytics: canAnalytics.value })
    } else if (canAnalytics.value) {
      await portfolioStore.loadPortfolio()
    }
  } catch {
    portfolioStore.selectClient(previousClientId)
    portfolioStore.setError('Não foi possível trocar o contexto de postagens.')
  } finally {
    internalAccountSwitch = false
    portfolioStore.setSwitching(false)
  }
}

function reload(): void {
  if (!scope.value) {
    void initializePublishingContext()
    return
  }
  if (portfolioMode.value) {
    if (canAnalytics.value) void portfolioStore.loadPortfolio()
    return
  }
  if (initialized.value) void store.refreshWorkspace()
  else void store.initialize({ includeAnalytics: canAnalytics.value })
}

watch(
  () => [canView.value, canAnalytics.value, accountStore.activeAccountId] as const,
  () => {
    closeComposer()
    if (!internalAccountSwitch) void initializePublishingContext()
  },
  { immediate: true },
)
watch(posts, (nextPosts) => {
  const selectedId = selectedPost.value?.id
  if (!composerOpen.value || !selectedId) return
  const current = nextPosts.find((post) => post.id === selectedId)
  if (current) selectedPost.value = current
})
watch(tabs, (options) => {
  if (!options.some((tab) => tab.id === activeTab.value)) activeTab.value = 'queue'
})
</script>

<template>
  <main class="sp-workspace">
    <SocialPublishingWorkspaceHeader
      :can-view="canView"
      :can-manage="canManage"
      :can-connect="canConnect"
      :can-select-client="scope?.canSelect === true"
      :individual-mode="individualMode"
      :connected="connection?.connected === true"
      :username="connection?.username || ''"
      :selected-client-id="selectedClientId"
      :client-options="clientOptions"
      :switching="switching"
      :loading-scope="loadingScope"
      @select-client="selectPublishingClient"
      @new-publication="openComposer()"
      @open-connection="activeTab = 'connection'"
    />

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
      <div v-if="scopeError" class="sp-workspace__error" role="alert">
        <UIcon name="i-lucide-circle-alert" aria-hidden="true" />
        <span>{{ scopeError }}</span>
        <UButton
          type="button"
          color="error"
          variant="ghost"
          size="sm"
          label="Tentar novamente"
          @click="reload"
        />
      </div>

      <div
        v-if="loadingScope || switching || !scopeResolved"
        class="sp-workspace__loading"
        aria-live="polite"
      >
        <span class="sp-workspace__spinner" aria-hidden="true"></span>
        <p>{{ switching ? 'Trocando cliente…' : 'Carregando clientes de postagens…' }}</p>
      </div>

      <section
        v-else-if="portfolioMode && !canAnalytics"
        class="sp-workspace__state omni-glass"
        aria-labelledby="sp-analytics-required"
      >
        <UIcon name="i-lucide-chart-no-axes-column" aria-hidden="true" />
        <h2 id="sp-analytics-required">Analytics necessário para o consolidado</h2>
        <p>
          Selecione um cliente acima ou solicite a permissão
          <code>social_publishing.analytics</code>
          para visualizar todos os clientes.
        </p>
      </section>

      <SocialPublishingPortfolioView
        v-else-if="portfolioMode"
        :portfolio="portfolio"
        :loading="loadingPortfolio"
        :error="portfolioError"
        @retry="portfolioStore.loadPortfolio"
        @select="selectPublishingClient"
      />

      <template v-else-if="individualMode">
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
          <SocialPublishingSummaryCards :overview="overview" :connection="connection" />

          <div v-if="!connection?.connected" class="sp-workspace__connection-note">
            <div>
              <UIcon name="i-lucide-unplug" aria-hidden="true" />
              <span>
                O Instagram está desconectado. Rascunhos podem ser preparados, mas o envio depende
                de uma conexão ativa.
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
            <SocialPublishingPagedList
              v-if="activeTab === 'queue'"
              :posts="scheduledPosts"
              mode="queue"
              :page-index="queuePage"
              :page-size="SOCIAL_PUBLISHING_PAGE_SIZE"
              :has-next="queueHasNext"
              :loading="queueLoading"
              :refreshing="refreshing"
              :can-manage="canManage"
              :busy-post-ids="busyPostIds"
              @page="store.loadPage('queue', $event)"
              @refresh="store.refreshWorkspace"
              @edit="openComposer"
              @cancel="cancelPost"
              @retry="retryPost"
            />
            <SocialPublishingPagedList
              v-else-if="activeTab === 'content'"
              :posts="contentPosts"
              mode="content"
              :page-index="contentPage"
              :page-size="SOCIAL_PUBLISHING_PAGE_SIZE"
              :has-next="contentHasNext"
              :loading="contentLoading"
              :refreshing="refreshing"
              :can-manage="canManage"
              :busy-post-ids="busyPostIds"
              @page="store.loadPage('content', $event)"
              @refresh="store.refreshWorkspace"
              @edit="openComposer"
              @cancel="cancelPost"
              @retry="retryPost"
            />
            <SocialPublishingAnalytics
              v-else-if="activeTab === 'analytics' && canAnalytics"
              :overview="overview"
              :posts="posts"
              :syncing="analyticsSyncing"
              :pending="analyticsSyncPending"
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
    </template>

    <SocialPublishingComposerDrawer
      v-if="canManage && individualMode"
      v-model="composerOpen"
      :post="selectedPost"
      :busy="savingPost || Boolean(selectedPost && busyPostIds.includes(selectedPost.id))"
      :error="error"
      :can-schedule="connection?.connected === true"
      @save="savePost"
      @schedule="schedulePost"
    />
  </main>
</template>

<style scoped src="./SocialPublishingWorkspace.css"></style>
