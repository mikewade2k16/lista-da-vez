<script setup lang="ts">
import type { CustomerRelationshipProfile } from '~/domain/customer-data/profile-types'

defineProps<{ profile: CustomerRelationshipProfile | null; relationshipId: string }>()
</script>

<template>
  <header class="ci-profile-header">
    <div>
      <p>Perfil por relacionamento</p>
      <h2>
        {{ profile?.relationship.displayName || profile?.subject.displayName || 'Cliente' }}
      </h2>
      <span>{{ profile?.relationship.lifecycleStatus || 'Sem classificacao' }}</span>
    </div>
    <dl>
      <div>
        <dt>Relacionamento</dt>
        <dd>{{ relationshipId }}</dd>
      </div>
      <div>
        <dt>Cliente</dt>
        <dd>{{ profile?.relationship.clientAccountId || 'Escopo atual' }}</dd>
      </div>
      <div>
        <dt>Revisao</dt>
        <dd>{{ profile?.relationship.revision ?? '—' }}</dd>
      </div>
    </dl>
  </header>
</template>

<style scoped>
.ci-profile-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
}

.ci-profile-header p,
.ci-profile-header span,
.ci-profile-header dt {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.ci-profile-header h2 {
  margin: 0.2rem 0;
}

.ci-profile-header dl {
  display: grid;
  gap: 0.4rem;
  min-width: min(24rem, 100%);
  margin: 0;
}

.ci-profile-header dl div {
  display: grid;
  grid-template-columns: 8rem minmax(0, 1fr);
  gap: 0.5rem;
}

.ci-profile-header dd {
  margin: 0;
  overflow-wrap: anywhere;
  font-size: 0.78rem;
}
</style>
