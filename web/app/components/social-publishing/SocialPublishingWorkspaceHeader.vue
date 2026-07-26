<script setup lang="ts">
import AppSelectField from '~/components/ui/AppSelectField.vue'

interface ClientOption {
  value: string
  label: string
}

defineProps<{
  canView: boolean
  canManage: boolean
  canConnect: boolean
  canSelectClient: boolean
  individualMode: boolean
  connected: boolean
  username: string
  selectedClientId: string
  clientOptions: ClientOption[]
  switching: boolean
  loadingScope: boolean
}>()

const emit = defineEmits<{
  'select-client': [clientId: string]
  'new-publication': []
  'open-connection': []
}>()

const aboutOpen = ref(false)
</script>

<template>
  <div class="sp-workspace__top">
    <AdminPageHeader
      eyebrow="Instagram"
      title="Agendamento de postagens"
      description="Prepare, agende e acompanhe publicações do cliente em um só lugar."
    />
    <div class="sp-workspace__actions">
      <AppSelectField
        v-if="canView && canSelectClient"
        class="sp-workspace__client-select"
        :model-value="selectedClientId"
        :options="clientOptions"
        placeholder="Todos os clientes"
        search-placeholder="Buscar cliente"
        :show-leading-icon="false"
        compact
        :disabled="switching || loadingScope"
        @update:model-value="emit('select-client', $event)"
      />
      <UButton
        v-if="canView"
        type="button"
        color="neutral"
        variant="soft"
        icon="i-lucide-circle-help"
        label="Como funciona"
        @click="aboutOpen = true"
      />
      <UButton
        v-if="canView && canManage && individualMode"
        type="button"
        color="primary"
        icon="i-lucide-plus"
        label="Nova publicação"
        :disabled="switching"
        @click="emit('new-publication')"
      />
    </div>

    <SocialPublishingAboutModal
      v-if="canView"
      v-model:open="aboutOpen"
      :individual-mode="individualMode"
      :connected="connected"
      :username="username"
      :can-open-connection="canConnect"
      @open-connection="emit('open-connection')"
    />
  </div>
</template>

<style scoped>
.sp-workspace__top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.sp-workspace__actions {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: flex-end;
  gap: 0.65rem;
}

.sp-workspace__client-select {
  width: min(18rem, 42vw);
}

@media (max-width: 640px) {
  .sp-workspace__top,
  .sp-workspace__actions {
    align-items: stretch;
    flex-direction: column;
  }

  .sp-workspace__client-select {
    width: 100%;
  }
}
</style>
