<script setup lang="ts">
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useKnowledgeIngestion } from '../../composables/accounting/useKnowledgeIngestion'
import { confirmFiscalAction } from '../../utils/fiscal-confirm'
const props = defineProps<{ issuerId: string; baseId: string }>()
const ingestion = useKnowledgeIngestion(
    () => props.issuerId,
    () => props.baseId,
  ),
  { state } = ingestion
const states: Record<string, string> = {
  queued: 'Aguardando coleta',
  fetching: 'Coletando',
  retry_wait: 'Nova tentativa agendada',
  review_pending: 'Versão preparada para revisão',
  unchanged: 'Conteúdo sem alteração',
  failed: 'Falha na coleta ou extração',
  cancelled: 'Coleta cancelada',
}
const errors: Record<string, string> = {
  source_fetch_unavailable: 'Fonte temporariamente indisponível',
  source_http_rejected: 'A fonte recusou o acesso; use importação manual permitida',
  source_type_or_size_rejected: 'Formato ou tamanho fora do limite',
  extraction_failed:
    'PDF não extraído integralmente; revise o formato ou importe uma cópia compatível',
  storage_unavailable: 'Armazenamento indisponível',
  permission_revoked: 'Acesso revogado',
  subscription_changed: 'Configuração alterada ou pausada',
  source_configuration_changed: 'Fonte mudou de configuração; revise a seleção',
  attempts_exhausted: 'Tentativas esgotadas',
  lease_expired: 'Execução anterior interrompida',
}
function select(event: Event) {
  ingestion.select((event.target as HTMLSelectElement).value)
}
async function save() {
  const token = `${props.issuerId}:${props.baseId}:${state.sourceId}:${state.enabled}:${state.intervalHours}`
  if (
    await confirmFiscalAction(
      'Salvar esta configuração de coleta periódica? Os documentos continuarão exigindo revisão e publicação manual.',
      () =>
        `${props.issuerId}:${props.baseId}:${state.sourceId}:${state.enabled}:${state.intervalHours}`,
    )
  ) {
    if (
      token ===
      `${props.issuerId}:${props.baseId}:${state.sourceId}:${state.enabled}:${state.intervalHours}`
    )
      await ingestion.save()
  }
}
</script>
<template>
  <details v-if="ingestion.allowed()" class="knowledge-ingestion">
    <summary>Atualização de fontes permitidas</summary>
    <p>
      A coleta fica desligada até você habilitar uma fonte. Novas versões não são publicadas
      automaticamente. Limite de 2 MiB e 10 páginas por PDF.
    </p>
    <AppPanelButton :disabled="state.busy" @click="ingestion.load()">
      Consultar fontes e coletas
    </AppPanelButton>
    <form v-if="ingestion.allowed(true)" class="knowledge-ingestion__form" @submit.prevent="save">
      <label>
        Fonte autorizada
        <select
          class="omni-input"
          :value="state.sourceId"
          :disabled="state.busy"
          required
          @change="select"
        >
          <option disabled value="">Escolha uma fonte da lista</option>
          <option v-for="source in state.catalog" :key="source.id" :value="source.id">
            {{ source.title }}
          </option>
        </select>
      </label>
      <label>
        <input v-model="state.enabled" type="checkbox" :disabled="state.busy" />
        Habilitar coleta periódica nesta base
      </label>
      <label>
        Intervalo em horas (6 a 168)
        <input
          v-model.number="state.intervalHours"
          class="omni-input"
          type="number"
          min="6"
          max="168"
          step="1"
          required
          :disabled="state.busy"
        />
      </label>
      <AppPanelButton type="submit" :disabled="state.busy || !state.sourceId">
        Salvar configuração
      </AppPanelButton>
    </form>
    <p v-if="state.error" role="alert">{{ state.error }}</p>
    <p v-if="state.busy" role="status">Consultando o servidor...</p>
    <p
      v-for="source in state.subscriptions.filter((s) => s.baseId === props.baseId)"
      :key="source.id"
    >
      {{ state.catalog.find((s) => s.id === source.sourceId)?.title || 'Fonte monitorada' }} ·
      {{ source.enabled ? 'Habilitada' : 'Pausada' }} · a cada {{ source.intervalHours }} horas
    </p>
    <article v-for="run in state.runs.filter((r) => r.baseId === props.baseId)" :key="run.id">
      <strong>{{ states[run.state] || 'Verifique a execução' }}</strong>
      <p v-if="run.errorCode">
        {{
          errors[run.errorCode] ||
          'Não foi possível concluir. Revise o acesso, a fonte e o armazenamento.'
        }}
      </p>
      <p v-if="run.state === 'review_pending'">
        Consulte os documentos desta base para revisar a versão e sua vigência antes de publicar.
      </p>
    </article>
    <AppPanelButton v-if="state.more" :disabled="state.busy" @click="ingestion.load(true)">
      Carregar mais coletas
    </AppPanelButton>
  </details>
</template>
<style scoped>
.knowledge-ingestion__form {
  display: grid;
  gap: 0.5rem;
  margin-block: 0.75rem;
}
.knowledge-ingestion__form label {
  display: grid;
  gap: 0.25rem;
}
.knowledge-ingestion article {
  border-top: 1px solid rgb(var(--border));
  padding-block: 0.5rem;
}
</style>
