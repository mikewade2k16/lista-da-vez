<script setup lang="ts">
import { Bell, MessageCircle, X } from 'lucide-vue-next'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useAuthStore } from '~/stores/auth'
import { useFeedbackStore } from '~/stores/feedback'
import { useUiStore } from '~/stores/ui'
import { useCoreAccountStore } from '../../../layers/core/stores/account'

const feedbackStore = useFeedbackStore()
const auth = useAuthStore()
const ui = useUiStore()
const accountStore = useCoreAccountStore()
const { allowedWorkspaces, storeContext, user } = storeToRefs(auth)
// Feedback e feature do modulo queue (ver module-enabled.global.ts e o gating do
// backend em /v1/feedback). Conta sem o modulo (ex.: so cardapio/bio) nao tem o
// recurso: nao busca (evitava 403 module_disabled em loop no polling) nem mostra
// o sino.
const hasFeedbackModule = computed(() => accountStore.enabledModules.includes('queue'))
const menuRef = ref(null)
const menuOpen = ref(false)
const feedbackSyncCursor = ref('')
let pollingTimer = null

const ownUserId = computed(() => String(user.value?.id || '').trim())
const canManageFeedback = computed(() => allowedWorkspaces.value.includes('feedback'))
const feedbackCollection = computed(() =>
  canManageFeedback.value ? feedbackStore.feedbacks : feedbackStore.myFeedbacks,
)
const feedbackPath = computed(() => (canManageFeedback.value ? '/feedback' : '/meus-feedbacks'))

function statusLabel(status) {
  const labels = {
    open: 'Aberto',
    in_progress: 'Em analise',
    resolved: 'Resolvido',
    closed: 'Fechado',
  }

  return labels[String(status || '').trim()] || status || '-'
}

function getStoreLabel(storeId) {
  const normalizedStoreId = String(storeId || '').trim()
  const store = (storeContext.value || []).find(
    (entry) => String(entry?.id || '').trim() === normalizedStoreId,
  )
  if (!store) {
    return 'Loja nao informada'
  }

  return String(store.name || store.code || store.city || 'Loja nao informada').trim()
}

function isUnreadForViewer(feedback, message, readAt) {
  const authorUserId = String(message.author_user_id || '').trim()
  const createdAt = new Date(message.created_at).getTime()
  if (!Number.isFinite(createdAt) || createdAt <= readAt) {
    return false
  }

  if (canManageFeedback.value) {
    return authorUserId === String(feedback.user_id || '').trim()
  }

  return authorUserId !== ownUserId.value
}

function isNewFeedbackUnread(feedback) {
  if (!canManageFeedback.value) {
    return false
  }

  const readAt = new Date(feedback.user_last_read_at || feedback.created_at).getTime()
  const createdAt = new Date(feedback.created_at).getTime()
  if (!Number.isFinite(createdAt) || createdAt <= readAt) {
    return false
  }

  return !(feedbackStore.messagesByFeedbackId[feedback.id] || []).length
}

function getLatestUnreadReply(feedback) {
  const readAt = new Date(feedback.user_last_read_at || feedback.created_at).getTime()
  const messages = feedbackStore.messagesByFeedbackId[feedback.id] || []

  return [...messages].reverse().find((message) => isUnreadForViewer(feedback, message, readAt))
}

const notifications = computed(() => {
  return feedbackCollection.value
    .map((feedback) => {
      const latestReply = getLatestUnreadReply(feedback)
      const unreadNewFeedback = isNewFeedbackUnread(feedback)

      if ((!latestReply && !unreadNewFeedback) || feedback.status === 'closed') {
        return null
      }

      const createdAt = unreadNewFeedback
        ? feedback.created_at || feedback.updated_at
        : latestReply.created_at || feedback.updated_at
      const preview = unreadNewFeedback
        ? feedback.body || ''
        : latestReply.body || feedback.body || ''

      return {
        id: unreadNewFeedback ? `${feedback.id}:created` : `${feedback.id}:${latestReply.id}`,
        feedbackId: feedback.id,
        title: feedback.subject || 'Chamado sem assunto',
        meta: canManageFeedback.value
          ? `${feedback.user_name || 'Usuario'} · ${getStoreLabel(feedback.store_id)}`
          : statusLabel(feedback.status),
        preview,
        createdAt,
        path: `${feedbackPath.value}?id=${encodeURIComponent(feedback.id)}`,
      }
    })
    .filter(Boolean)
    .sort((left, right) => new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime())
    .slice(0, 6)
})

const notificationCount = computed(() => notifications.value.length)

function isDocumentVisible() {
  return !import.meta.client || document.visibilityState === 'visible'
}

function toggleMenu() {
  menuOpen.value = !menuOpen.value
}

function closeMenu() {
  menuOpen.value = false
}

function formatDate(isoString) {
  try {
    return new Date(isoString).toLocaleDateString('pt-BR', {
      day: '2-digit',
      month: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return ''
  }
}

async function dismissNotification(notification) {
  const result = await feedbackStore.markFeedbackAsRead(notification.feedbackId)
  if (!result.ok) {
    ui.error(result.message || 'Erro ao apagar notificacao')
  }
}

function handleNotificationOpen(notification) {
  closeMenu()
  void feedbackStore.markFeedbackAsRead(notification.feedbackId)
}

async function loadNotifications() {
  if (!hasFeedbackModule.value || !ownUserId.value || !isDocumentVisible()) {
    return
  }

  const result = canManageFeedback.value
    ? await feedbackStore.fetchFeedbacks(
        feedbackSyncCursor.value ? { since: feedbackSyncCursor.value } : undefined,
      )
    : await feedbackStore.fetchMyFeedbacks(
        feedbackSyncCursor.value ? { since: feedbackSyncCursor.value } : undefined,
      )

  if (!result.ok) {
    return
  }

  if (result.cursor) {
    feedbackSyncCursor.value = result.cursor
  }

  await feedbackStore.syncMessagesForFeedbacks(
    feedbackCollection.value.map((feedback) => feedback.id),
  )
}

function startPolling() {
  stopPolling()
  pollingTimer = window.setInterval(loadNotifications, 30000)
}

function stopPolling() {
  if (pollingTimer) {
    window.clearInterval(pollingTimer)
    pollingTimer = null
  }
}

function handlePointerDown(event) {
  if (!menuOpen.value) {
    return
  }

  if (menuRef.value && !menuRef.value.contains(event.target)) {
    closeMenu()
  }
}

function handleVisibilityChange() {
  if (isDocumentVisible()) {
    void loadNotifications()
  }
}

watch(
  [ownUserId, canManageFeedback, hasFeedbackModule],
  () => {
    feedbackSyncCursor.value = ''
    void loadNotifications()
  },
  { immediate: true },
)

onMounted(() => {
  startPolling()
  document.addEventListener('pointerdown', handlePointerDown)
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  stopPolling()
  document.removeEventListener('pointerdown', handlePointerDown)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<template>
  <div v-if="hasFeedbackModule" ref="menuRef" class="feedback-notifications">
    <button
      class="feedback-notifications__trigger"
      type="button"
      aria-label="Abrir notificacoes"
      aria-haspopup="menu"
      :aria-expanded="menuOpen ? 'true' : 'false'"
      @click="toggleMenu"
    >
      <Bell :size="18" :stroke-width="2.15" />
      <span v-if="notificationCount" class="feedback-notifications__badge">
        {{ notificationCount }}
      </span>
    </button>

    <Transition name="feedback-notifications-menu">
      <div v-if="menuOpen" class="feedback-notifications__dropdown" role="menu">
        <header class="feedback-notifications__header">
          <strong>Notificacoes</strong>
          <NuxtLink :to="feedbackPath" @click="closeMenu">Ver chamados</NuxtLink>
        </header>

        <div v-if="notifications.length" class="feedback-notifications__list">
          <div
            v-for="notification in notifications"
            :key="notification.id"
            class="feedback-notifications__item"
          >
            <NuxtLink
              class="feedback-notifications__item-link"
              :to="notification.path"
              role="menuitem"
              @click="handleNotificationOpen(notification)"
            >
              <span class="feedback-notifications__icon">
                <MessageCircle :size="15" :stroke-width="2.2" />
              </span>
              <span class="feedback-notifications__copy">
                <strong>{{ notification.title }}</strong>
                <small>{{ notification.meta }}</small>
                <span>{{ notification.preview }}</span>
                <small>{{ formatDate(notification.createdAt) }}</small>
              </span>
            </NuxtLink>

            <button
              class="feedback-notifications__dismiss"
              type="button"
              :aria-label="`Apagar notificacao de ${notification.title}`"
              @click.stop="dismissNotification(notification)"
            >
              <X :size="14" :stroke-width="2.2" />
            </button>
          </div>
        </div>

        <NuxtLink
          v-else
          :to="feedbackPath"
          class="feedback-notifications__empty feedback-notifications__empty-link"
          role="menuitem"
          @click="closeMenu"
        >
          <strong>Nenhuma resposta nova</strong>
          <span>Quando responderem seus chamados, eles aparecem aqui.</span>
        </NuxtLink>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.feedback-notifications {
  position: relative;
}

.feedback-notifications__trigger {
  position: relative;
  display: grid;
  place-items: center;
  width: 3rem;
  height: 3rem;
  padding: 0;
  border: 1px solid rgb(var(--border) / 0.9);
  border-radius: 999px;
  background: rgb(var(--surface) / 0.86);
  color: rgb(var(--text));
  cursor: pointer;
}

.feedback-notifications__trigger:hover,
.feedback-notifications__trigger[aria-expanded='true'] {
  border-color: rgb(var(--primary) / 0.42);
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
}

.feedback-notifications__badge {
  position: absolute;
  top: 0.35rem;
  right: 0.35rem;
  min-width: 1rem;
  height: 1rem;
  padding: 0 0.25rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: rgb(var(--danger));
  color: rgb(var(--surface));
  font-size: 0.62rem;
  font-weight: 800;
}

.feedback-notifications__dropdown {
  position: absolute;
  top: calc(100% + 0.55rem);
  right: 0;
  z-index: 35;
  width: min(23rem, calc(100vw - 2rem));
  display: grid;
  gap: 0.65rem;
  padding: 0.8rem;
  border: 1px solid rgb(var(--border) / 0.9);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.98);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(18px);
}

.feedback-notifications__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.1rem 0.2rem 0.35rem;
  color: rgb(var(--text));
}

.feedback-notifications__header a {
  color: rgb(var(--primary));
  font-size: 0.75rem;
  font-weight: 800;
  text-decoration: none;
}

.feedback-notifications__list {
  display: grid;
  gap: 0.45rem;
}

.feedback-notifications__item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.55rem;
  align-items: start;
}

.feedback-notifications__item-link {
  width: 100%;
  min-width: 0;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 0.65rem;
  padding: 0.72rem;
  border: 1px solid rgb(var(--border) / 0.78);
  border-radius: 0.85rem;
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
  text-decoration: none;
}

.feedback-notifications__item-link:hover {
  border-color: rgb(var(--primary) / 0.28);
  background: rgb(var(--primary) / 0.1);
}

.feedback-notifications__dismiss {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  margin-top: 0.2rem;
  border: 1px solid rgb(var(--border) / 0.78);
  border-radius: 999px;
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
  cursor: pointer;
}

.feedback-notifications__dismiss:hover {
  border-color: rgb(var(--danger) / 0.35);
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
}

.feedback-notifications__icon {
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
}

.feedback-notifications__copy {
  min-width: 0;
  display: grid;
  gap: 0.18rem;
}

.feedback-notifications__copy strong,
.feedback-notifications__copy span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.feedback-notifications__copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.feedback-notifications__copy strong {
  font-size: 0.82rem;
}

.feedback-notifications__copy span {
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.feedback-notifications__copy small {
  color: rgb(var(--primary));
  font-size: 0.68rem;
}

.feedback-notifications__empty {
  display: grid;
  gap: 0.25rem;
  padding: 1rem;
  border-radius: 0.85rem;
  border: 1px solid rgb(var(--border) / 0.72);
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
  font-size: 0.78rem;
}

.feedback-notifications__empty strong {
  color: rgb(var(--text));
}

.feedback-notifications__empty-link {
  text-decoration: none;
}

.feedback-notifications__empty-link:hover {
  border-color: rgb(var(--primary) / 0.28);
  background: rgb(var(--primary) / 0.1);
}

.feedback-notifications-menu-enter-active,
.feedback-notifications-menu-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}

.feedback-notifications-menu-enter-from,
.feedback-notifications-menu-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
