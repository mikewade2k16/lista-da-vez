<script setup lang="ts">
// Fase 9 (apply-operacao): esqueleto exibido ENQUANTO o realtime/snapshot da
// operacao conecta. E aditivo — aparece no estado loading da pagina e some
// quando os dados chegam (isRemoteRosterReady). Nao substitui nenhum
// comportamento de realtime/faixa de consultores; apenas evita tela vazia.
withDefaults(
  defineProps<{
    scopeMode?: string
  }>(),
  {
    scopeMode: 'single',
  },
)
</script>

<template>
  <section class="operation-skeleton" aria-busy="true" aria-live="polite">
    <span class="operation-skeleton__sr">
      {{
        scopeMode === 'all'
          ? 'Sincronizando a operacao integrada das lojas acessiveis.'
          : 'Sincronizando consultores, fila e atendimento da loja ativa.'
      }}
    </span>

    <header class="operation-skeleton__scopebar">
      <CoreSkeleton variant="block" width="11rem" height="2.4rem" />
      <CoreSkeleton variant="block" width="8rem" height="2.4rem" />
    </header>

    <div class="operation-skeleton__main">
      <div class="operation-skeleton__columns">
        <article v-for="column in 2" :key="`col-${column}`" class="operation-skeleton__column">
          <CoreSkeleton variant="text" width="40%" height="0.9rem" />
          <CoreSkeleton variant="card" :count="3" />
        </article>
      </div>

      <aside class="operation-skeleton__strip" aria-hidden="true">
        <CoreSkeleton variant="text" width="35%" height="0.9rem" />
        <div class="operation-skeleton__avatars">
          <span v-for="avatar in 6" :key="`avatar-${avatar}`" class="operation-skeleton__avatar">
            <CoreSkeleton variant="avatar" />
            <CoreSkeleton variant="text" width="80%" height="0.7rem" />
          </span>
        </div>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.operation-skeleton {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  width: 100%;
}

.operation-skeleton__sr {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.operation-skeleton__scopebar {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.operation-skeleton__main {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 1rem;
}

.operation-skeleton__columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}

.operation-skeleton__column {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  padding: 0.9rem;
  border: 1px solid var(--line-soft);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
}

.operation-skeleton__strip {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
  padding: 0.9rem;
  border: 1px solid var(--line-soft);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
}

.operation-skeleton__avatars {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(7rem, 1fr));
  gap: 0.9rem;
}

.operation-skeleton__avatar {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
}

@media (max-width: 900px) {
  .operation-skeleton__columns {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
