<script setup>
import { computed, watch } from 'vue'
import DemoWorkspacePage from '~/components/demo/DemoWorkspacePage.vue'
import TenantsWorkspace from '~/components/tenants/TenantsWorkspace.vue'
import UsersWorkspace from '~/components/users/UsersWorkspace.vue'
import { useAuthStore } from '~/stores/auth'
import { useTenantsStore } from '~/stores/tenants'
import { useUsersStore } from '~/stores/users'
import { getDemoPage } from '~/utils/demo-pages'

function normalizeManageArea(value) {
  return String(value || '')
    .trim()
    .toLowerCase()
}

function resolveManageAreaWorkspaceId(area) {
  if (area === 'users' || area === 'usuarios') {
    return 'usuarios'
  }

  if (area === 'clientes' || area === 'clients') {
    return 'clientes'
  }

  return 'manage'
}

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'manage',
  pageLabel: 'Manage',
  middleware: [
    (to) => {
      const auth = useAuthStore()
      const allowedWorkspaces = new Set(auth.allowedWorkspaces || [])
      const requiredWorkspaceId = resolveManageAreaWorkspaceId(normalizeManageArea(to.params.area))

      if (!allowedWorkspaces.has('manage') || !allowedWorkspaces.has(requiredWorkspaceId)) {
        return navigateTo(auth.homePath, { replace: true })
      }

      return undefined
    },
  ],
})

const route = useRoute()
const auth = useAuthStore()
const usersStore = useUsersStore()
const tenantsStore = useTenantsStore()

const area = computed(() => normalizeManageArea(route.params.area))

const manageWorkspace = computed(() => {
  if (area.value === 'users' || area.value === 'usuarios') {
    return {
      kind: 'users',
    }
  }

  if (area.value === 'clientes' || area.value === 'clients') {
    return {
      kind: 'clientes',
      legacyRoute: '/clientes',
      note: 'Frente nova de gestao de clientes/tenants. A rota operacional atual da fila continua disponivel em /clientes.',
    }
  }

  return null
})

const page = computed(() => getDemoPage(`manage/${route.params.area}`))

watch(
  () => area.value,
  (currentArea) => {
    const allowedWorkspaces = new Set(auth.allowedWorkspaces || [])
    const requiredWorkspaceId = resolveManageAreaWorkspaceId(currentArea)

    if (!allowedWorkspaces.has('manage') || !allowedWorkspaces.has(requiredWorkspaceId)) {
      void navigateTo(auth.homePath, { replace: true })
    }
  },
  { immediate: true },
)

watch(
  () => manageWorkspace.value?.kind,
  (kind) => {
    if (kind === 'users') {
      void usersStore.ensureLoaded()
      return
    }

    if (kind === 'clientes') {
      void tenantsStore.ensureLoaded()
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="page-workspace">
    <template v-if="manageWorkspace?.kind === 'users'">
      <UsersWorkspace mode="admin" />
    </template>

    <template v-else-if="manageWorkspace?.kind === 'clientes'">
      <article class="insight-card">
        <p class="settings-card__text">
          {{ manageWorkspace.note }}
          <NuxtLink :to="manageWorkspace.legacyRoute">Abrir versao da fila</NuxtLink>
          .
        </p>
      </article>

      <TenantsWorkspace />
    </template>

    <DemoWorkspacePage v-else :page="page" />
  </div>
</template>
