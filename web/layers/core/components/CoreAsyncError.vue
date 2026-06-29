<script setup lang="ts">
import { onMounted } from 'vue'
import CoreErrorState from './CoreErrorState.vue'

// errorComponent de um defineAsyncComponent (ver ~/utils/lazy-component). Renderizado
// no lugar de um componente lazy quando o carregamento do chunk falha de vez. Sem isso,
// a falha vira tela em branco silenciosa (ver references/registro-de-falhas.md #10).
// Reusa CoreErrorState (sem duplicar UI) e oferece recarregar a pagina.
const props = defineProps<{ error?: unknown }>()

onMounted(() => {
  // Consome a prop `error` (a causa real da falha) e a loga pra diagnostico —
  // tambem evita o fallthrough dela como atributo no elemento raiz.
  if (import.meta.client && props.error) {
    console.error('[lazy-component] falha ao carregar chunk:', props.error)
  }
})

function reload() {
  if (import.meta.client) window.location.reload()
}
</script>

<template>
  <CoreErrorState
    title="Nao foi possivel carregar esta secao"
    message="A versao do app pode ter mudado ou a conexao falhou. Recarregue para tentar de novo."
    retry-label="Recarregar"
    compact
    @retry="reload"
  />
</template>
