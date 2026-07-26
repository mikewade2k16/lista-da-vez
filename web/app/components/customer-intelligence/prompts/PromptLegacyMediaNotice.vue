<script setup lang="ts">
import type { LegacyManagedCapability } from '~/domain/customer-intelligence/prompt-types'

defineProps<{ capabilities: LegacyManagedCapability[] }>()

function safeLegacyLink(value: string | undefined): string {
  const normalized = String(value ?? '').trim()
  return /^\/omnichannel(?:[/?#]|$)/.test(normalized) ? normalized : ''
}
</script>

<template>
  <section v-if="capabilities.length" class="legacy-media">
    <h3>Legado gerenciado no Omnichannel</h3>
    <p>Audio/transcricao e video permanecem read-only aqui e nao viram process keys novas.</p>
    <div v-for="capability in capabilities" :key="capability.key">
      <span>{{ capability.key }} · {{ capability.owner }} · {{ capability.status }}</span>
      <NuxtLink
        v-if="safeLegacyLink(capability.deepLink)"
        :to="safeLegacyLink(capability.deepLink)"
      >
        Configurar no Omnichannel
      </NuxtLink>
    </div>
  </section>
</template>

<style scoped>
.legacy-media {
  display: grid;
  gap: 0.5rem;
  padding: 0.8rem;
  border: 1px solid rgb(var(--warning) / 0.3);
  border-radius: 0.8rem;
  background: rgb(var(--warning) / 0.06);
}

.legacy-media h3,
.legacy-media p {
  margin: 0;
}

.legacy-media div {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
}

.legacy-media p,
.legacy-media span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}
</style>
