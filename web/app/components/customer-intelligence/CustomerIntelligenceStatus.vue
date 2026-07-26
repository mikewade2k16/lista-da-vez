<script setup lang="ts">
import type { CustomerApiErrorState } from '~/domain/customer-intelligence/api-error'

const props = withDefaults(
  defineProps<{
    title?: string
    error?: CustomerApiErrorState | null
    loading?: boolean
    empty?: boolean
    emptyText?: string
  }>(),
  {
    title: '',
    error: null,
    loading: false,
    empty: false,
    emptyText: 'Nenhum dado disponivel neste escopo.',
  },
)

const message = computed(() => {
  if (!props.error) return ''
  switch (props.error.kind) {
    case 'capability_off':
      return 'Esta funcionalidade esta desligada para o escopo atual.'
    case 'forbidden':
      return 'Sua permissao atual nao permite consultar este conteudo.'
    case 'not_found':
      return 'O recurso nao existe ou nao pertence ao escopo selecionado.'
    case 'unavailable':
      return 'O servico ainda nao esta disponivel neste ambiente.'
    default:
      return props.error.message
  }
})
</script>

<template>
  <section
    v-if="loading || error || empty"
    class="ci-status"
    :class="{ 'ci-status--error': error }"
    role="status"
  >
    <strong>
      {{ title || (loading ? 'Carregando' : error ? 'Conteudo indisponivel' : 'Sem dados') }}
    </strong>
    <span v-if="loading">Aguarde enquanto o escopo autorizado e consultado.</span>
    <span v-else-if="error">{{ message }}</span>
    <span v-else>{{ emptyText }}</span>
    <small v-if="error?.reasonCode">Codigo: {{ error.reasonCode }}</small>
  </section>
</template>

<style scoped>
.ci-status {
  display: grid;
  gap: 0.35rem;
  min-height: 8rem;
  place-content: center;
  padding: 1.25rem;
  border: 1px dashed rgb(var(--border) / 0.9);
  border-radius: 1rem;
  background: rgb(var(--surface-2) / 0.55);
  color: rgb(var(--muted));
  text-align: center;
}

.ci-status strong {
  color: rgb(var(--text));
}

.ci-status--error {
  border-color: rgb(var(--warning) / 0.35);
}

.ci-status small {
  font-size: 0.7rem;
}
</style>
