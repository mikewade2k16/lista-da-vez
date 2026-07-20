<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'
import { fetchCapabilities } from '~/domain/omnichannel/config-api'
import type { OmniCapabilities } from '~/domain/omnichannel/config-types'

// Readout das capacidades DAQUELE numero (a UI degrada por numero — canonico §12 risco 2).
// REGRA: capability desconhecida = AUSENTE. O back ainda nao expoe o endpoint de
// capabilities (needsWiring); enquanto isso o fetch devolve null e a tela avisa que nao
// confirmou nada — nunca oferece um recurso que o numero pode nao ter.
const props = defineProps<{ instanceId: string }>()

const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const caps = ref<OmniCapabilities | null>(null)
const loading = ref(false)
const loaded = ref(false)

async function load(): Promise<void> {
  loading.value = true
  loaded.value = false
  caps.value = await fetchCapabilities(api, props.instanceId)
  loading.value = false
  loaded.value = true
}

onMounted(() => void load())
watch(
  () => props.instanceId,
  () => void load(),
)

// Cada linha e binaria: presente (true) ou ausente/desconhecido (false). Nunca inferimos
// do provider — so refletimos o que o numero confirmou.
const rows = computed(() => {
  const c = caps.value
  return [
    { label: 'Templates (mensagem fora da janela)', on: Boolean(c?.supportsTemplates) },
    { label: 'Exige janela de 24h', on: Boolean(c?.requires24hWindow) },
    { label: 'Reação', on: Boolean(c?.supportsReaction) },
    { label: 'Sticker', on: Boolean(c?.supportsSticker) },
    { label: 'Grupos', on: Boolean(c?.supportsGroups) },
  ]
})

const maxMediaMb = computed(() => {
  const bytes = caps.value?.maxMediaBytes || 0
  if (bytes <= 0) return ''
  return `${Math.round(bytes / (1024 * 1024))} MB`
})
</script>

<template>
  <div class="cfg-caps">
    <p v-if="loading" class="cfg-caps__hint">Consultando capacidades do número…</p>

    <template v-else-if="loaded && !caps">
      <p class="cfg-caps__warn">
        <span class="cfg-caps__warn-tag">DEGRADADO</span>
        As capacidades deste número ainda não são confirmadas pelo servidor. O painel não oferece
        recursos opcionais (templates, reação, sticker) até o número confirmar que os suporta —
        capacidade desconhecida é tratada como ausente.
      </p>
    </template>

    <template v-else>
      <ul class="cfg-caps__list">
        <li v-for="row in rows" :key="row.label" class="cfg-caps__item">
          <span
            class="cfg-caps__dot"
            :class="row.on ? 'is-on' : 'is-off'"
            aria-hidden="true"
          ></span>
          <span class="cfg-caps__label">{{ row.label }}</span>
          <span class="cfg-caps__state">{{ row.on ? 'sim' : 'não' }}</span>
        </li>
        <li v-if="maxMediaMb" class="cfg-caps__item">
          <span class="cfg-caps__dot is-on" aria-hidden="true"></span>
          <span class="cfg-caps__label">Mídia até</span>
          <span class="cfg-caps__state">{{ maxMediaMb }}</span>
        </li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.cfg-caps {
  display: grid;
  gap: 0.5rem;
}

.cfg-caps__hint {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.78rem;
}

.cfg-caps__warn {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin: 0;
  padding: 0.5rem 0.65rem;
  border: 1px solid color-mix(in srgb, var(--accent-warning) 45%, transparent);
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--accent-warning) 12%, transparent);
  color: rgb(var(--text));
  font-size: 0.78rem;
  line-height: 1.35;
}

.cfg-caps__warn-tag {
  flex: none;
  padding: 0.1rem 0.35rem;
  border-radius: var(--radius-xs);
  background: var(--accent-warning);
  color: rgb(var(--text));
  font-size: 0.66rem;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.cfg-caps__list {
  display: grid;
  gap: 0.3rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.cfg-caps__item {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8rem;
  color: rgb(var(--text));
}

.cfg-caps__dot {
  width: 0.55rem;
  height: 0.55rem;
  border-radius: 999px;
}

.cfg-caps__dot.is-on {
  background: rgb(var(--success));
}

.cfg-caps__dot.is-off {
  background: rgb(var(--border));
}

.cfg-caps__state {
  color: rgb(var(--muted));
  font-weight: 700;
  font-size: 0.74rem;
}
</style>
