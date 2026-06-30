<script setup lang="ts">
// Acoes em lote POR MODULO, exibidas no TOPO DO CORPO do collapse de cada modulo
// (acima da lista de permissoes daquele modulo), na aba "Modulos". "Permitir todos /
// Negar todos / Herdar todos" setam o tri-estado de todas as permissoes VISIVEIS
// daquele modulo (respeitando busca/filtro). "Restaurar" reverte as edicoes
// pendentes do modulo para o ultimo estado salvo. Nada aqui auto-salva — so muda o
// rascunho/dirty; o "Salvar modulos" existente persiste.

type BulkEffect = 'allow' | 'deny' | 'inherit'

defineProps<{
  // Quantas permissoes visiveis deste modulo o lote vai afetar.
  visibleCount: number
  // Ha edicao pendente neste modulo (habilita "Restaurar").
  dirty: boolean
  // Trava durante carregamento/salvamento.
  disabled?: boolean
}>()

const emit = defineEmits<{
  apply: [BulkEffect]
  restore: []
}>()
</script>

<template>
  <div class="module-actions" role="group" aria-label="Acoes em lote do modulo">
    <span class="module-actions__label">Em lote:</span>
    <button
      type="button"
      class="module-actions__btn module-actions__btn--allow"
      title="Permitir todas as permissoes visiveis deste modulo"
      :disabled="disabled || visibleCount === 0"
      @click="emit('apply', 'allow')"
    >
      Permitir todos
    </button>
    <button
      type="button"
      class="module-actions__btn module-actions__btn--deny"
      title="Negar todas as permissoes visiveis deste modulo"
      :disabled="disabled || visibleCount === 0"
      @click="emit('apply', 'deny')"
    >
      Negar todos
    </button>
    <button
      type="button"
      class="module-actions__btn"
      title="Herdar (sem override) em todas as permissoes visiveis deste modulo"
      :disabled="disabled || visibleCount === 0"
      @click="emit('apply', 'inherit')"
    >
      Herdar todos
    </button>
    <button
      type="button"
      class="module-actions__btn module-actions__btn--restore"
      title="Reverter as edicoes pendentes deste modulo para o ultimo estado salvo"
      :disabled="disabled || !dirty"
      @click="emit('restore')"
    >
      Restaurar
    </button>
  </div>
</template>

<style scoped>
.module-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.3rem;
  padding: 0.4rem 0.5rem;
  border-radius: var(--radius-sm);
  border: 1px dashed rgb(var(--border));
  background: rgb(var(--surface));
}

.module-actions__label {
  margin-right: 0.15rem;
  font-size: 0.72rem;
  font-weight: 700;
  color: rgb(var(--muted));
}

.module-actions__btn {
  padding: 0.2rem 0.5rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.7rem;
  font-weight: 600;
  cursor: pointer;
}

.module-actions__btn:hover:not(:disabled) {
  border-color: rgb(var(--primary) / 0.45);
}

.module-actions__btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.module-actions__btn--allow {
  border-color: rgb(var(--success) / 0.4);
  color: rgb(var(--success));
}

.module-actions__btn--deny {
  border-color: rgb(var(--danger) / 0.4);
  color: rgb(var(--danger));
}

.module-actions__btn--restore {
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary-600));
}
</style>
