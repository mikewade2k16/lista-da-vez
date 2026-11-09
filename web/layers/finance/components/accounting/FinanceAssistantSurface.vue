<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import FinanceConversation from '../fiscal/FinanceConversation.vue'
import AccountingWorkspace from './AccountingWorkspace.vue'
import CrowPlusConversation from './CrowPlusConversation.vue'
import { useCrowPlus } from '../../composables/useCrowPlus'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useFinanceAccountContext } from '../../composables/finance-account-context'
import { useAuthStore } from '~/stores/auth'
const emit = defineEmits<{ discuss: [text: string] }>()
const plus = useCrowPlus()
const account = useFinanceAccountContext(),
  auth = useAuthStore()
const enterprise = computed(
  () =>
    account.enabledModules.includes('accounting') &&
    account.permissions.includes('accounting.issuers.view'),
)
const selected = ref<'personal' | 'enterprise' | 'archive'>('personal')
const management = ref(false)
watch(
  () => `${account.activeAccountId}:${auth.user?.id}:${enterprise.value}`,
  () => {
    selected.value = 'personal'
    management.value = false
  },
  { flush: 'sync' },
)
</script>
<template>
  <div class="finance-assistant-surface">
    <div
      v-if="enterprise"
      class="finance-assistant-surface__tabs"
      role="group"
      aria-label="Contexto do atendimento"
    >
      <AppPanelButton
        :variant="selected === 'personal' ? 'primary' : 'secondary'"
        @click="selected = 'personal'"
      >
        Conversa
      </AppPanelButton>
      <AppPanelButton
        :variant="selected === 'enterprise' ? 'primary' : 'secondary'"
        @click="selected = 'enterprise'"
      >
        Empresarial
      </AppPanelButton>
    </div>
    <AppPanelButton
      v-if="plus.state.enabled"
      variant="secondary"
      @click="selected = selected === 'archive' ? 'personal' : 'archive'"
    >
      {{ selected === 'archive' ? 'Voltar à conversa' : 'Histórico de consultas anterior' }}
    </AppPanelButton>
    <CrowPlusConversation
      v-if="selected === 'archive'"
      :controller="plus"
      readonly
      @personal="selected = 'personal'"
    />
    <template v-else-if="selected === 'enterprise' && enterprise">
      <AppPanelButton variant="secondary" @click="management = !management">
        {{ management ? 'Voltar à conversa' : 'Abrir cadastros e revisão' }}
      </AppPanelButton>
      <AccountingWorkspace v-if="management" :key="`${account.activeAccountId}:${auth.user?.id}`" />
      <CrowPlusConversation
        v-else-if="plus.state.enabled"
        :controller="plus"
        enterprise-only
        @personal="selected = 'personal'"
      />
      <p v-else>
        As ferramentas empresariais precisam ser habilitadas na configuração do Finance. O cadastro
        continua disponível pelo botão acima.
      </p>
    </template>
    <FinanceConversation v-else @discuss="emit('discuss', $event)" />
  </div>
</template>
<style scoped>
.finance-assistant-surface {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  flex: 1;
}
.finance-assistant-surface__tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  padding: 0.5rem;
}
</style>
