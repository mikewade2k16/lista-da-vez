<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '~/stores/auth'

// Marcador de legado/mock/nao-persistido. Visivel SOMENTE para platform_admin
// (regra AGENT_RULES "Legado, mocks e fonte da verdade"): sinaliza na propria
// tela que aquilo ainda nao e' a fonte de verdade, para ninguem desenvolver
// achando que esta pronto. Cada uso deve apontar para a entrada em docs/LEGADO.md.
const props = withDefaults(
  defineProps<{
    // O que e' legado/mock nesta tela.
    label: string
    // 'legacy' = tabela/codigo legado; 'mock' = dado falso; 'localstorage' = nao persiste.
    kind?: 'legacy' | 'mock' | 'localstorage'
    // Referencia/explicacao curta (ex.: "usa user_tenant_roles — ver docs/LEGADO.md #1").
    detail?: string
  }>(),
  { kind: 'legacy', detail: '' },
)

const auth = useAuthStore()
const visible = computed(() => auth.role === 'platform_admin')
const badge = computed(() =>
  props.kind === 'mock' ? 'MOCK' : props.kind === 'localstorage' ? 'localStorage' : 'LEGADO',
)
</script>

<template>
  <div v-if="visible" class="legacy-marker" :class="`legacy-marker--${kind}`" role="note">
    <span class="legacy-marker__badge">{{ badge }}</span>
    <span class="legacy-marker__label">{{ label }}</span>
    <span v-if="detail" class="legacy-marker__detail">{{ detail }}</span>
  </div>
</template>

<style scoped>
.legacy-marker {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  padding: 0.4rem 0.7rem;
  border-radius: 0.6rem;
  border: 1px dashed rgb(var(--warning) / 0.5);
  background: rgb(var(--warning) / 0.1);
  font-size: 0.76rem;
}

.legacy-marker--mock {
  border-color: rgb(var(--danger) / 0.5);
  background: rgb(var(--danger) / 0.1);
}

.legacy-marker__badge {
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: color-mix(in srgb, rgb(var(--warning)) 70%, rgb(var(--text)) 30%);
}

.legacy-marker--mock .legacy-marker__badge {
  color: color-mix(in srgb, rgb(var(--danger)) 70%, rgb(var(--text)) 30%);
}

.legacy-marker__label {
  color: var(--text-main);
  font-weight: 600;
}

.legacy-marker__detail {
  color: var(--text-muted);
}
</style>
