import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { useAuthStore } from '~/stores/auth'

const DEFAULT_CLIENT_OPTIONS = [
  { label: 'crow', value: 106 },
  { label: 'Perola', value: 101 },
  { label: 'Dr Antonio Tavares', value: 104 },
  { label: 'UNO', value: 105 },
]

export const useTasksClientStore = defineStore('tasks-client', () => {
  const auth = useAuthStore()

  // Derived from real auth role. consultant = client viewer; everyone else = admin.
  const userType = computed<'admin' | 'client'>(() =>
    auth.role === 'consultant' ? 'client' : 'admin',
  )
  const userLevel = computed(() => auth.role || 'admin')

  // TODO tasks-refactor-v2: clientId e clientOptions virão da API de contatos do CRM.
  // O campo clientId em tasks é legado (integer), requer migração de backend antes de usar UUID.
  const clientId = ref(106)
  const clientOptions = ref([...DEFAULT_CLIENT_OPTIONS])
  const loadingClientOptions = ref(false)
  const clientOptionsSynced = ref(false)

  const isAdmin = computed(() => userType.value === 'admin')
  const activeClientLabel = computed(
    () =>
      clientOptions.value.find((client) => client.value === clientId.value)?.label ||
      `Cliente #${clientId.value}`,
  )

  function initialize() {
    clientOptionsSynced.value = true
  }

  async function refreshClientOptions() {
    // TODO tasks-refactor-v2: buscar da API real de contatos/accounts
    loadingClientOptions.value = true
    clientOptions.value = [...DEFAULT_CLIENT_OPTIONS]
    clientOptionsSynced.value = true
    loadingClientOptions.value = false
  }

  function setClientId(nextClientId: number | string) {
    const parsed = Number(nextClientId)
    if (Number.isFinite(parsed) && parsed > 0) {
      clientId.value = parsed
    }
  }

  return {
    userType,
    userLevel,
    clientId,
    clientOptions,
    loadingClientOptions,
    clientOptionsSynced,
    isAdmin,
    activeClientLabel,
    initialize,
    refreshClientOptions,
    setClientId,
  }
})
