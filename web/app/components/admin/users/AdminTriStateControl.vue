<script setup lang="ts">
// Controle tri-estado compartilhado (Herdar / Permitir / Negar) usado nos pain:
// Modulos e Paginas do editor de usuario. Tres botoes segmentados; o do meio
// (allow) fica verde quando ativo, o ultimo (deny) vermelho, o primeiro (inherit)
// na cor primaria. Os rotulos sao configuraveis (ex.: Mostrar/Ocultar nas paginas)
// mas a semantica e fixa: inherit = sem override, allow, deny.
//
// Emite `update` com o novo estado; o pai e dono do valor (states[key]). Centraliza
// a marcacao + estilo dos botoes para os dois paineis nao duplicarem CSS.

type TriState = 'inherit' | 'allow' | 'deny'

withDefaults(
  defineProps<{
    modelValue: TriState
    // Rotulo de cada estado. Default = vocabulario de Modulos.
    inheritLabel?: string
    allowLabel?: string
    denyLabel?: string
    ariaLabel?: string
  }>(),
  {
    inheritLabel: 'Herdar',
    allowLabel: 'Permitir',
    denyLabel: 'Negar',
    ariaLabel: undefined,
  },
)

const emit = defineEmits<{ update: [TriState] }>()
</script>

<template>
  <div class="tri-control" role="group" :aria-label="ariaLabel">
    <button
      type="button"
      class="tri-control__btn"
      :class="{ 'is-active': modelValue === 'inherit' }"
      @click="emit('update', 'inherit')"
    >
      {{ inheritLabel }}
    </button>
    <button
      type="button"
      class="tri-control__btn tri-control__btn--allow"
      :class="{ 'is-active': modelValue === 'allow' }"
      @click="emit('update', 'allow')"
    >
      {{ allowLabel }}
    </button>
    <button
      type="button"
      class="tri-control__btn tri-control__btn--deny"
      :class="{ 'is-active': modelValue === 'deny' }"
      @click="emit('update', 'deny')"
    >
      {{ denyLabel }}
    </button>
  </div>
</template>

<style scoped>
.tri-control {
  display: inline-flex;
  flex-shrink: 0;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  overflow: hidden;
}

.tri-control__btn {
  padding: 0.3rem 0.6rem;
  font-size: 0.74rem;
  border: none;
  background: rgb(var(--surface));
  color: rgb(var(--muted));
  cursor: pointer;
}

.tri-control__btn + .tri-control__btn {
  border-left: 1px solid rgb(var(--border));
}

.tri-control__btn.is-active {
  background: rgb(var(--primary));
  color: rgb(var(--surface));
}

.tri-control__btn--allow.is-active {
  background: rgb(var(--success));
}

.tri-control__btn--deny.is-active {
  background: rgb(var(--danger));
}
</style>
