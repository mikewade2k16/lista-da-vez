<script setup lang="ts">
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import type { OmniAiToolApproval, OmniAiToolRun } from '~/domain/omnichannel/config-types'

defineProps<{
  runs: OmniAiToolRun[]
  approvals: OmniAiToolApproval[]
  canManage: boolean
  canAudit: boolean
  busy: boolean
}>()

const emit = defineEmits<{
  refresh: []
  approve: [approval: OmniAiToolApproval]
  reject: [approval: OmniAiToolApproval]
}>()

function formatJSON(value: Record<string, unknown>): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return '{}'
  }
}
</script>

<template>
  <section class="tools-card">
    <div class="tools-card__head">
      <strong>Evidências e aprovações</strong>
      <div class="runs-head-actions">
        <span>{{ approvals.length }} propostas · {{ runs.length }} execuções</span>
        <AppPanelButton variant="ghost" :disabled="busy" @click="emit('refresh')">
          Atualizar
        </AppPanelButton>
      </div>
    </div>
    <p class="tools-muted">
      Argumentos e resultados são projeções mascaradas do Go. Aprovar apenas autoriza o retry
      assinado; nenhuma chamada externa é feita pelo navegador.
    </p>
    <div v-if="canAudit" class="tool-evidence">
      <strong>Evidências de execução</strong>
      <div v-for="run in runs" :key="run.id" class="tool-run">
        <div class="tool-run__summary">
          <div>
            <strong>{{ run.toolId }} · {{ run.operation }}</strong>
            <small>
              {{ run.status }} · {{ run.latencyMs }}ms ·
              {{ new Date(run.createdAt).toLocaleString() }}
            </small>
          </div>
        </div>
        <div class="tool-run__payloads">
          <pre><code>{{ formatJSON(run.inputMasked) }}</code></pre>
          <pre><code>{{ formatJSON(run.outputMasked) }}</code></pre>
        </div>
        <small v-if="run.error" class="tool-run__error">{{ run.error }}</small>
      </div>
      <p v-if="!runs.length" class="tools-muted">Nenhuma execução registrada.</p>
    </div>
    <div v-if="canManage" class="tool-evidence">
      <strong>Propostas de ação mutável</strong>
    </div>
    <div v-for="approval in approvals" :key="approval.id" class="tool-run">
      <div class="tool-run__summary">
        <div>
          <strong>{{ approval.toolId }} · {{ approval.operation }}</strong>
          <small>
            {{ approval.status }} · {{ approval.latencyMs }}ms ·
            {{ new Date(approval.requestedAt).toLocaleString() }}
          </small>
        </div>
        <div v-if="approval.status === 'pending'" class="tool-run__actions">
          <AppPanelButton :disabled="!canManage || busy" @click="emit('approve', approval)">
            Aprovar
          </AppPanelButton>
          <AppPanelButton
            variant="danger"
            :disabled="!canManage || busy"
            @click="emit('reject', approval)"
          >
            Negar
          </AppPanelButton>
        </div>
      </div>
      <div class="tool-run__payloads">
        <pre><code>{{ formatJSON(approval.inputMasked) }}</code></pre>
      </div>
      <small v-if="approval.reason || approval.error" class="tool-run__error">
        {{ approval.reason || approval.error }}
      </small>
    </div>
    <p v-if="!approvals.length" class="tools-muted">Nenhuma proposta de tool registrada.</p>
  </section>
</template>

<style scoped>
.runs-head-actions,
.tool-run__summary,
.tool-run__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.tool-run {
  display: grid;
  gap: 0.5rem;
  padding-top: 0.7rem;
  border-top: 1px solid var(--line-soft);
}
.tool-evidence {
  display: grid;
  gap: 0.5rem;
}
.tool-run__summary {
  justify-content: space-between;
}
.tool-run__summary > div:first-child {
  display: grid;
  gap: 0.15rem;
}
.tool-run small,
.tool-run__error {
  color: var(--text-muted);
  font-size: 0.72rem;
}
.tool-run__error {
  color: var(--danger, #ef8c8c);
}
.tool-run__payloads {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
}
.tool-run pre {
  max-height: 8rem;
  margin: 0;
  overflow: auto;
  padding: 0.55rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface) / 0.6);
  color: var(--text-muted);
  font-size: 0.68rem;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
@media (max-width: 780px) {
  .tool-run__summary,
  .runs-head-actions {
    align-items: flex-start;
    flex-direction: column;
  }
  .tool-run__payloads {
    grid-template-columns: 1fr;
  }
}
</style>
