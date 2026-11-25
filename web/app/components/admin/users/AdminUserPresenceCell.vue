<script setup lang="ts">
import { formatAge } from '~/utils/observability/health'
import type { AdminUserItem } from '~/types/admin-users'

// Coluna "Ultimo acesso" de /manage/users (so platform_admin). Online = sinal
// (uso da sessao ou heartbeat do front) nos ultimos 2 minutos, calculado no backend.
const props = defineProps<{ user: AdminUserItem }>()

const dateFormatter = new Intl.DateTimeFormat('pt-BR', {
  dateStyle: 'short',
  timeStyle: 'short',
})

function formatDate(iso: string | null) {
  if (!iso) return 'nunca'
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? 'nunca' : dateFormatter.format(date)
}

const label = computed(() => {
  if (props.user.isOnline) return 'Online'
  return props.user.lastSeenAt ? formatAge(props.user.lastSeenAt) : 'Nunca acessou'
})

const title = computed(
  () =>
    `Ultimo acesso: ${formatDate(props.user.lastSeenAt)}\nUltimo login: ${formatDate(props.user.lastLoginAt)}`,
)
</script>

<template>
  <span
    class="admin-user-presence"
    :class="{ 'admin-user-presence--online': user.isOnline }"
    :title="title"
  >
    <span class="admin-user-presence__dot" aria-hidden="true"></span>
    <span class="admin-user-presence__label">{{ label }}</span>
  </span>
</template>

<style scoped>
.admin-user-presence {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: rgb(var(--muted));
  white-space: nowrap;
}

.admin-user-presence__dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 999px;
  background: rgb(var(--border));
  flex-shrink: 0;
}

.admin-user-presence--online {
  color: rgb(var(--success));
}

.admin-user-presence--online .admin-user-presence__dot {
  background: rgb(var(--success));
}
</style>
