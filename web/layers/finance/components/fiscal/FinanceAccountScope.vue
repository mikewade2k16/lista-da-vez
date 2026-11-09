<script setup lang="ts">
import { computed, ref } from 'vue'
import CalendarChatScope from '~/components/calendar/CalendarChatScope.vue'
import { useCoreAccountStore } from '../../../core/stores/account'
const props = defineProps<{ disabled?: boolean; chatOnly?: boolean; targetId?: string }>()
const emit = defineEmits<{ select: [id: string] }>()
const account = useCoreAccountStore()
const switching = ref(false),
  error = ref('')
const scope = computed(() => ({
  canSelect: account.accounts.length > 1,
  lockedClientId: account.activeAccountId,
  clients: account.accounts.map((item) => ({
    id: item.id,
    name: `${item.name}${item.isAgency ? ' (Agência)' : ''}${!(item.modules || []).includes('finance') ? ' — Finance não habilitado' : ''}`,
    disabled: !(item.modules || []).includes('finance'),
  })),
}))
async function select(_mode: 'client' | 'all', id: string) {
  if (
    props.disabled ||
    switching.value ||
    !id ||
    id === (props.chatOnly ? props.targetId : account.activeAccountId)
  )
    return
  const target = account.accounts.find((item) => item.id === id)
  if (!target || !target.modules.includes('finance')) return
  if (props.chatOnly) {
    emit('select', id)
    return
  }
  switching.value = true
  error.value = ''
  try {
    await account.switchAccount(id)
    if (!account.context) error.value = 'Não foi possível carregar o acesso a esse cliente.'
  } catch {
    error.value = 'Não foi possível trocar de cliente. Confira seu acesso e tente novamente.'
  } finally {
    switching.value = false
  }
}
</script>
<template>
  <CalendarChatScope
    :scope="scope"
    mode="client"
    :client-id="chatOnly ? targetId || '' : account.activeAccountId"
    :allow-all="false"
    label="Cliente em atendimento no Finance"
    :disabled="disabled || switching || account.loading"
    @change="select"
  />
  <p v-if="error" class="calendar-chat__error" role="alert">{{ error }}</p>
</template>
