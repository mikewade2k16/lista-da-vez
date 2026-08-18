<script setup lang="ts">
import {
  Bell,
  BellRing,
  CheckCircle2,
  ClipboardCheck,
  MessageCircle,
  RefreshCw,
  ShieldAlert,
  X,
} from 'lucide-vue-next'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useDropdownDismiss } from '~/composables/useDropdownDismiss'
import type {
  SystemNotificationBucket,
  SystemNotificationItem,
} from '~/domain/system-notifications/system-notifications'
import { useSystemNotificationsStore } from '~/stores/system-notifications'

type FilterKey = 'all' | SystemNotificationBucket

const notifications = useSystemNotificationsStore()
const root = ref<HTMLElement | null>(null)
const open = ref(false)
const activeFilter = ref<FilterKey>('all')

const sourceDefinitions: Array<{
  key: SystemNotificationBucket
  label: string
}> = [
  { key: 'system', label: 'Sistema' },
  { key: 'feedback', label: 'Chamados' },
  { key: 'content', label: 'Conteúdo' },
  { key: 'operations', label: 'Operação' },
]

const sourceFilters = computed(() =>
  sourceDefinitions
    .map((source) => ({
      ...source,
      count: notifications.items.filter((item) => item.bucket === source.key).length,
    }))
    .filter((source) => source.count > 0),
)
const filteredItems = computed(() =>
  notifications.items
    .filter((item) => activeFilter.value === 'all' || item.bucket === activeFilter.value)
    .slice(0, 15),
)

function close(): void {
  open.value = false
}

function toggle(): void {
  open.value = !open.value
  if (open.value) void notifications.refresh({ forceContent: true })
}

function formatDate(value: string): string {
  if (!value) return ''
  const dateOnly = value.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (dateOnly) return `${dateOnly[3]}/${dateOnly[2]}/${dateOnly[1]}`
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return ''
  return parsed.toLocaleDateString('pt-BR', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function selectFilter(filter: FilterKey): void {
  activeFilter.value = filter
}

function openItem(item: SystemNotificationItem): void {
  close()
  if (item.dismissible) void notifications.markRead(item)
}

async function dismiss(item: SystemNotificationItem): Promise<void> {
  await notifications.markRead(item)
}

useDropdownDismiss(() => open.value, close, { rootRef: root })

onMounted(() => notifications.start())
onBeforeUnmount(() => notifications.stop())
</script>

<template>
  <div v-if="notifications.enabled" ref="root" class="system-notifications">
    <button
      class="system-notifications__trigger"
      type="button"
      aria-label="Abrir notificações"
      aria-haspopup="menu"
      :aria-expanded="open"
      @click="toggle"
    >
      <Bell :size="18" :stroke-width="2.15" aria-hidden="true" />
      <span v-if="notifications.count" class="system-notifications__badge">
        {{ notifications.count > 99 ? '99+' : notifications.count }}
      </span>
    </button>

    <Transition name="system-notifications-menu">
      <section v-if="open" class="system-notifications__dropdown" aria-label="Notificações">
        <header class="system-notifications__header">
          <div>
            <small>Central do sistema</small>
            <strong>Notificações</strong>
          </div>
          <button
            class="system-notifications__refresh"
            type="button"
            aria-label="Atualizar notificações"
            :disabled="notifications.loading"
            @click="notifications.refresh({ forceContent: true })"
          >
            <RefreshCw :size="16" :class="{ 'is-spinning': notifications.loading }" />
          </button>
        </header>

        <nav
          v-if="sourceFilters.length"
          class="system-notifications__filters"
          aria-label="Filtrar por origem"
        >
          <button
            type="button"
            :class="{ 'is-active': activeFilter === 'all' }"
            @click="selectFilter('all')"
          >
            Todas
            <span>{{ notifications.count }}</span>
          </button>
          <button
            v-for="source in sourceFilters"
            :key="source.key"
            type="button"
            :class="{ 'is-active': activeFilter === source.key }"
            @click="selectFilter(source.key)"
          >
            {{ source.label }}
            <span>{{ source.count }}</span>
          </button>
        </nav>

        <div v-if="filteredItems.length" class="system-notifications__list" role="menu">
          <article
            v-for="item in filteredItems"
            :key="item.id"
            class="system-notifications__item"
            :class="`is-${item.severity}`"
          >
            <NuxtLink :to="item.linkPath" role="menuitem" @click="openItem(item)">
              <span class="system-notifications__icon" :class="`is-${item.bucket}`">
                <MessageCircle v-if="item.bucket === 'feedback'" :size="16" />
                <ClipboardCheck v-else-if="item.bucket === 'content'" :size="16" />
                <ShieldAlert v-else-if="item.bucket === 'operations'" :size="16" />
                <BellRing v-else :size="16" />
              </span>
              <span class="system-notifications__copy">
                <span class="system-notifications__meta-row">
                  <b>{{ item.sourceLabel }}</b>
                  <time v-if="formatDate(item.occurredAt)">{{ formatDate(item.occurredAt) }}</time>
                </span>
                <strong>{{ item.title }}</strong>
                <span v-if="item.body">{{ item.body }}</span>
                <small v-if="item.meta">{{ item.meta }}</small>
              </span>
            </NuxtLink>
            <button
              v-if="item.dismissible"
              class="system-notifications__dismiss"
              type="button"
              :aria-label="`Marcar ${item.title} como lida`"
              @click="dismiss(item)"
            >
              <X :size="14" />
            </button>
          </article>
        </div>

        <div v-else class="system-notifications__empty">
          <CheckCircle2 :size="24" aria-hidden="true" />
          <strong>Nada pendente por aqui</strong>
          <span>Novos alertas dos módulos aparecerão neste sino.</span>
        </div>
      </section>
    </Transition>
  </div>
</template>

<style scoped>
.system-notifications {
  position: relative;
}
.system-notifications__trigger {
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
.system-notifications__trigger:hover,
.system-notifications__trigger[aria-expanded='true'] {
  border-color: rgb(var(--primary) / 0.42);
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
}
.system-notifications__badge {
  position: absolute;
  top: 0.28rem;
  right: 0.2rem;
  display: grid;
  place-items: center;
  min-width: 1.12rem;
  height: 1.12rem;
  padding: 0 0.22rem;
  border-radius: 999px;
  background: rgb(var(--danger));
  color: rgb(var(--surface));
  font-size: 0.6rem;
  font-weight: 900;
}
.system-notifications__dropdown {
  position: absolute;
  top: calc(100% + 0.55rem);
  right: 0;
  z-index: 40;
  width: min(27rem, calc(100vw - 1.25rem));
  display: grid;
  gap: 0.7rem;
  padding: 0.85rem;
  border: 1px solid rgb(var(--border) / 0.9);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.99);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(18px);
}
.system-notifications__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.05rem 0.15rem 0.15rem;
}
.system-notifications__header > div {
  display: grid;
  gap: 0.08rem;
}
.system-notifications__header small {
  color: rgb(var(--muted));
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.system-notifications__header strong {
  font-size: 1rem;
}
.system-notifications__refresh {
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border: 1px solid rgb(var(--border) / 0.75);
  border-radius: 999px;
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
  cursor: pointer;
}
.system-notifications__refresh:hover {
  color: rgb(var(--primary));
}
.system-notifications__refresh:disabled {
  opacity: 0.6;
  cursor: wait;
}
.system-notifications__filters {
  display: flex;
  gap: 0.35rem;
  max-width: 100%;
  overflow-x: auto;
  padding-bottom: 0.1rem;
  scrollbar-width: none;
}
.system-notifications__filters::-webkit-scrollbar {
  display: none;
}
.system-notifications__filters button {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  flex: 0 0 auto;
  min-height: 1.9rem;
  padding: 0 0.58rem;
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: 999px;
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
  font-size: 0.68rem;
  font-weight: 800;
  cursor: pointer;
}
.system-notifications__filters button span {
  display: grid;
  place-items: center;
  min-width: 1.1rem;
  height: 1.1rem;
  padding: 0 0.22rem;
  border-radius: 999px;
  background: rgb(var(--surface));
  font-size: 0.6rem;
}
.system-notifications__filters button.is-active {
  border-color: rgb(var(--primary) / 0.35);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}
.system-notifications__list {
  display: grid;
  max-height: min(31rem, calc(100vh - 13rem));
  overflow-y: auto;
  overscroll-behavior: contain;
}
.system-notifications__item {
  position: relative;
  border-top: 1px solid rgb(var(--border) / 0.68);
}
.system-notifications__item:first-child {
  border-top: 0;
}
.system-notifications__item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0.82rem;
  bottom: 0.82rem;
  width: 3px;
  border-radius: 999px;
  background: rgb(var(--primary));
}
.system-notifications__item.is-critical::before {
  background: rgb(var(--danger));
}
.system-notifications__item.is-warning::before {
  background: rgb(var(--warning));
}
.system-notifications__item > a {
  display: grid;
  grid-template-columns: 2rem minmax(0, 1fr);
  gap: 0.62rem;
  padding: 0.72rem 2.35rem 0.72rem 0.7rem;
  color: inherit;
  text-decoration: none;
}
.system-notifications__item > a:hover {
  background: rgb(var(--primary) / 0.055);
}
.system-notifications__icon {
  display: grid;
  place-items: center;
  width: 1.9rem;
  height: 1.9rem;
  border-radius: 0.62rem;
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}
.system-notifications__icon.is-feedback {
  background: rgb(var(--info) / 0.12);
  color: rgb(var(--info));
}
.system-notifications__icon.is-content {
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
}
.system-notifications__icon.is-operations {
  background: rgb(var(--warning) / 0.13);
  color: rgb(var(--warning));
}
.system-notifications__copy {
  min-width: 0;
  display: grid;
  gap: 0.16rem;
}
.system-notifications__copy > strong {
  font-size: 0.82rem;
  line-height: 1.3;
}
.system-notifications__copy > span:not(.system-notifications__meta-row) {
  display: -webkit-box;
  overflow: hidden;
  color: rgb(var(--muted));
  font-size: 0.74rem;
  line-height: 1.38;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}
.system-notifications__copy > small {
  overflow: hidden;
  color: rgb(var(--text));
  font-size: 0.67rem;
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.system-notifications__meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  min-width: 0;
}
.system-notifications__meta-row b {
  overflow: hidden;
  color: rgb(var(--primary));
  font-size: 0.62rem;
  letter-spacing: 0.035em;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}
.system-notifications__meta-row time {
  flex: 0 0 auto;
  color: rgb(var(--muted));
  font-size: 0.62rem;
}
.system-notifications__dismiss {
  position: absolute;
  top: 0.76rem;
  right: 0.3rem;
  display: grid;
  place-items: center;
  width: 1.8rem;
  height: 1.8rem;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: rgb(var(--muted));
  cursor: pointer;
}
.system-notifications__dismiss:hover {
  background: rgb(var(--danger) / 0.1);
  color: rgb(var(--danger));
}
.system-notifications__empty {
  display: grid;
  justify-items: center;
  gap: 0.3rem;
  padding: 1.5rem;
  text-align: center;
  color: rgb(var(--muted));
}
.system-notifications__empty svg {
  color: rgb(var(--success));
}
.system-notifications__empty strong {
  color: rgb(var(--text));
  font-size: 0.85rem;
}
.system-notifications__empty span {
  font-size: 0.72rem;
}
.system-notifications-menu-enter-active,
.system-notifications-menu-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}
.system-notifications-menu-enter-from,
.system-notifications-menu-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
.is-spinning {
  animation: system-notifications-spin 0.8s linear infinite;
}
@keyframes system-notifications-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (max-width: 900px) {
  .system-notifications__trigger {
    width: 2.35rem;
    height: 2.35rem;
  }
}
@media (max-width: 640px) {
  .system-notifications__trigger {
    width: 2.2rem;
    height: 2.2rem;
  }
  .system-notifications__dropdown {
    position: fixed;
    top: 4.25rem;
    right: 0.625rem;
    left: 0.625rem;
    width: auto;
    max-height: calc(100vh - 5rem);
  }
  .system-notifications__list {
    max-height: calc(100vh - 14rem);
  }
}
</style>
