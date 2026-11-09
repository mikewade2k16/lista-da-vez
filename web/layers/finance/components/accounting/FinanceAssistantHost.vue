<script setup lang="ts">
import { computed, onBeforeUnmount, provide, ref, watch } from 'vue'
import FinanceAssistantSurface from './FinanceAssistantSurface.vue'
import FinanceAccountScope from '../fiscal/FinanceAccountScope.vue'
import { FINANCE_CHAT_ACCOUNT } from '../../composables/finance-account-context'
import { useCoreAccountStore, type AccountContext } from '../../../core/stores/account'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'
const emit = defineEmits<{ discuss: [text: string] }>()
const root = useCoreAccountStore(),
  auth = useAuthStore()
const selection = useState<{ actorId: string; accountId: string }>('finance:chat-account', () => ({
  actorId: '',
  accountId: '',
}))
const actor = computed(
  () => auth.principal?.userId || auth.principal?.userID || auth.user?.id || '',
)
const accountId = computed(() => selection.value.accountId)
const context = ref<AccountContext | null>(null),
  loading = ref(false),
  error = ref('')
const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
provide(FINANCE_CHAT_ACCOUNT, { accountId, context })
watch(
  [actor, () => root.activeAccountId, () => root.accounts.map((a) => a.id).join(',')],
  () => {
    if (!actor.value) {
      selection.value = { actorId: '', accountId: '' }
      return
    }
    if (
      selection.value.actorId !== actor.value ||
      !root.accounts.some((a) => a.id === accountId.value)
    )
      selection.value = { actorId: actor.value, accountId: root.activeAccountId }
  },
  { immediate: true, flush: 'sync' },
)
let generation = 0
watch(
  [accountId, actor],
  async () => {
    const ticket = ++generation,
      id = accountId.value
    context.value = null
    error.value = ''
    loading.value = true
    if (!id || !actor.value) {
      loading.value = false
      return
    }
    try {
      const data = (await api(`/v2/me/context?accountId=${encodeURIComponent(id)}`, {
        headers: { 'X-Account-Id': id },
        retry: 0,
        dedupe: false,
        skipLoadingIndicator: true,
      })) as { context: AccountContext }
      if (ticket !== generation) return
      if (data.context?.account.id !== id) throw new Error('Contexto de cliente inválido')
      context.value = data.context
    } catch {
      if (ticket === generation)
        error.value = 'Não foi possível carregar o acesso a este cliente no chat.'
    } finally {
      if (ticket === generation) loading.value = false
    }
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  generation++
})
function select(id: string) {
  if (root.accounts.some((a) => a.id === id && a.modules.includes('finance')))
    selection.value = { actorId: actor.value, accountId: id }
}
</script>
<template>
  <div class="finance-assistant-host">
    <FinanceAccountScope chat-only :target-id="accountId" :disabled="loading" @select="select" />
    <p v-if="loading" class="calendar-chat__empty" role="status">
      Carregando contexto do cliente...
    </p>
    <p v-else-if="error" class="calendar-chat__error" role="alert">{{ error }}</p>
    <FinanceAssistantSurface
      v-else-if="context"
      :key="`${actor}:${accountId}`"
      @discuss="emit('discuss', $event)"
    />
  </div>
</template>
<style scoped>
.finance-assistant-host {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
</style>
