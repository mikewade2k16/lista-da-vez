<script setup lang="ts">
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import { useOfflineInteractions } from '~/composables/customer-intelligence/useOfflineInteractions'
import OfflineInteractionDrawer from './OfflineInteractionDrawer.vue'

const props = defineProps<{ relationshipId: string }>()
const offline = useOfflineInteractions(() => props.relationshipId)
const drawerOpen = ref(false)

async function save(input: Parameters<typeof offline.create>[0]): Promise<void> {
  const created = await offline.create(input)
  if (created) drawerOpen.value = false
}
</script>

<template>
  <section class="offline-timeline">
    <header>
      <div>
        <h2>Interacoes offline</h2>
        <p>Reunioes, ligacoes e contatos presenciais registrados no Customer Data.</p>
      </div>
      <button
        v-if="offline.access.canManageOffline.value && offline.descriptor.value"
        type="button"
        @click="drawerOpen = true"
      >
        Registrar
      </button>
    </header>
    <CustomerIntelligenceStatus
      v-if="offline.loading.value && !offline.items.value.length"
      title="Carregando interacoes offline"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="offline.error.value"
      title="Interacoes offline indisponiveis"
      :error="offline.error.value"
    />
    <CustomerIntelligenceStatus
      v-else-if="!offline.items.value.length"
      title="Sem interacoes offline"
      empty
      empty-text="Nenhum contato offline foi registrado neste relacionamento."
    />
    <ol v-else>
      <li v-for="interaction in offline.items.value" :key="interaction.id">
        <div>
          <small>{{ interaction.interactionType }} · {{ interaction.purposeKey }}</small>
          <strong>{{ interaction.title }}</strong>
          <p>{{ interaction.content }}</p>
        </div>
        <time :datetime="interaction.occurredAt">
          {{ new Date(interaction.occurredAt).toLocaleString('pt-BR') }}
        </time>
      </li>
    </ol>
    <button
      v-if="offline.nextCursor.value"
      type="button"
      :disabled="offline.loading.value"
      @click="offline.load(true)"
    >
      Carregar mais
    </button>
    <OfflineInteractionDrawer
      v-if="offline.descriptor.value"
      v-model:open="drawerOpen"
      :descriptor="offline.descriptor.value"
      :saving="offline.saving.value"
      @save="save"
    />
  </section>
</template>

<style scoped>
.offline-timeline {
  display: grid;
  gap: 0.8rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
}

.offline-timeline header,
.offline-timeline li {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
}

.offline-timeline h2,
.offline-timeline p {
  margin: 0;
}

.offline-timeline ol {
  display: grid;
  gap: 0.65rem;
  margin: 0;
  padding-left: 1.2rem;
}

.offline-timeline li div {
  display: grid;
  gap: 0.2rem;
}

.offline-timeline small,
.offline-timeline time,
.offline-timeline p {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}
</style>
