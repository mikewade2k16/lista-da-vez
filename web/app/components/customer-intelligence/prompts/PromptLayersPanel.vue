<script setup lang="ts">
defineProps<{ processKey: string }>()

const layers = [
  { key: 'platform_guardrail', label: 'Guardrail da plataforma', editable: false },
  { key: 'agency_policy', label: 'Politica textual da agencia', editable: true },
  { key: 'client_policy', label: 'Politica textual do cliente', editable: true },
  { key: 'process_prompt', label: 'Prompt do processo', editable: true },
  { key: 'agent_override', label: 'Override permitido do agente', editable: true },
  { key: 'runtime_context', label: 'Contexto minimizado em runtime', editable: false },
]
</script>

<template>
  <section class="prompt-layers">
    <h3>Camadas e precedencia</h3>
    <p>
      O processo
      <code>{{ processKey }}</code>
      usa composicao versionada. Uma camada textual nunca amplia source, tool, consentimento ou
      sender.
    </p>
    <ol>
      <li v-for="layer in layers" :key="layer.key">
        <strong>{{ layer.label }}</strong>
        <span>
          {{ layer.editable ? 'Configuravel por binding autorizado' : 'Invariante/read-only' }}
        </span>
      </li>
    </ol>
  </section>
</template>

<style scoped>
.prompt-layers {
  display: grid;
  gap: 0.6rem;
}

.prompt-layers h3,
.prompt-layers p {
  margin: 0;
}

.prompt-layers p,
.prompt-layers span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.prompt-layers ol {
  display: grid;
  gap: 0.35rem;
  margin: 0;
  padding-left: 1.25rem;
}

.prompt-layers li {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
}
</style>
