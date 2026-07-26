<script setup>
import { computed, watch } from 'vue'
import DemoWorkspacePage from '~/components/demo/DemoWorkspacePage.vue'
import UsersWorkspace from '~/components/users/UsersWorkspace.vue'
import { useAuthStore } from '~/stores/auth'
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

  return 'manage'
}

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'manage',
  pageLabel: 'Manage',
  middleware: [
    (to) => {
      const area = normalizeManageArea(to.params.area)
      if (area === 'clientes' || area === 'clients') {
        return navigateTo('/manage/clientes-web', { replace: true })
      }

      const auth = useAuthStore()
      const allowedWorkspaces = new Set(auth.allowedWorkspaces || [])
      const requiredWorkspaceId = resolveManageAreaWorkspaceId(area)

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

const area = computed(() => normalizeManageArea(route.params.area))

const manageWorkspace = computed(() => {
  if (area.value === 'users' || area.value === 'usuarios') {
    return {
      kind: 'users',
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
  },
  { immediate: true },
)
</script>

<template>
  <div class="page-workspace">
    <template v-if="manageWorkspace?.kind === 'users'">
      <UsersWorkspace mode="admin" />
    </template>

    <DemoWorkspacePage v-else :page="page" />
  </div>
</template>
