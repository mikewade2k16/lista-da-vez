<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AdminPageHeader from '../../../../layers/core/components/admin/AdminPageHeader.vue'
import PublicKnowledgeCollectionPanel from '~/components/admin/knowledge/PublicKnowledgeCollectionPanel.vue'
import { useAuthStore } from '~/stores/auth'
import { usePublicKnowledgeStore } from '~/stores/public-knowledge'
import { getApiErrorMessage } from '~/utils/api-client'
import { attemptLabel, catalogStatusLabel, reviewSummaryLabel } from '~/utils/public-knowledge'
import type { PublicCatalogStatus } from '~/types/public-knowledge'

// Base pública do chat: coleta de fontes oficiais do catálogo fechado, revisão
// (você, outras IAs, contador) e publicação. O conteúdo vale para todas as contas.
const auth = useAuthStore()
const store = usePublicKnowledgeStore()
const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
const items = ref<PublicCatalogStatus[]>([])
const loading = ref(false)
const collecting = ref('')
const error = ref('')
const status = ref('')
let controller: AbortController | null = null

async function load() {
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = ''
  try {
    items.value = await store.catalog(controller.signal)
  } catch (cause) {
    if (!controller.signal.aborted)
      error.value = getApiErrorMessage(cause, 'Não foi possível carregar o catálogo.')
  } finally {
    loading.value = false
  }
}
async function collect(keys: string[]) {
  error.value = ''
  status.value = ''
  const results: string[] = []
  for (const key of keys) {
    collecting.value = key
    try {
      const attempt = await store.collect(key)
      results.push(`${key}: ${attemptLabel(attempt)}`)
    } catch (cause) {
      results.push(`${key}: ${getApiErrorMessage(cause, 'falhou')}`)
    }
  }
  collecting.value = ''
  status.value = results.join(' · ')
  await load()
}
onMounted(() => {
  if (isPlatformAdmin.value) void load()
})
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <section class="public-knowledge">
    <AdminPageHeader
      eyebrow="Plataforma"
      title="Base pública do chat"
      description="Fontes oficiais consultadas pelo chat do Finance sem IA. Coletar gera um rascunho; nada aparece no chat até você aprovar e publicar."
    />
    <UAlert
      v-if="!isPlatformAdmin"
      color="error"
      variant="soft"
      icon="i-lucide-shield-alert"
      title="Acesso restrito"
      description="Esta área é exclusiva para administradores da plataforma."
    />
    <template v-else>
      <ol class="public-knowledge__steps">
        <li>Coletar: baixa a fonte oficial, extrai o texto e divide em trechos (rascunho).</li>
        <li>
          Revisar: confira os trechos, exclua os ruins e exporte o texto para outras IAs e para o
          contador.
        </li>
        <li>
          Registrar cada revisão. Publicar exige a sua aprovação; IA e contador aparecem no chat
          como revisados ou pendentes.
        </li>
      </ol>
      <div class="public-knowledge__toolbar">
        <button
          type="button"
          class="public-knowledge__button"
          :disabled="!!collecting || loading"
          @click="collect(items.map((item) => item.entry.key))"
        >
          {{ collecting ? `Coletando ${collecting}...` : 'Coletar todas as fontes' }}
        </button>
        <button type="button" class="public-knowledge__button" :disabled="loading" @click="load">
          Atualizar
        </button>
      </div>
      <p v-if="error" class="public-knowledge__status public-knowledge__status--error" role="alert">
        {{ error }}
      </p>
      <p v-if="status" class="public-knowledge__status" role="status">{{ status }}</p>
      <p v-if="loading && !items.length" class="public-knowledge__status" role="status">
        Carregando catálogo...
      </p>

      <details v-for="item in items" :key="item.entry.key" class="public-knowledge__source">
        <summary>
          <span class="public-knowledge__title">{{ item.entry.title }}</span>
          <span class="public-knowledge__badge">{{ catalogStatusLabel(item) }}</span>
        </summary>
        <p class="public-knowledge__line">
          {{ item.entry.authority }} · {{ item.entry.regime }} ·
          {{ item.entry.contentType === 'application/pdf' ? 'PDF' : 'página' }} ·
          <a :href="item.entry.url" target="_blank" rel="noopener noreferrer">fonte oficial</a>
        </p>
        <p class="public-knowledge__line">{{ item.entry.notes }}</p>
        <p class="public-knowledge__line">{{ attemptLabel(item.lastAttempt) }}</p>
        <p v-if="item.published" class="public-knowledge__line">
          No chat: versão {{ item.published.version }} —
          {{ reviewSummaryLabel(item.published.review) }}
        </p>
        <div class="public-knowledge__toolbar">
          <button
            type="button"
            class="public-knowledge__button"
            :disabled="!!collecting"
            @click="collect([item.entry.key])"
          >
            {{ collecting === item.entry.key ? 'Coletando...' : 'Coletar agora' }}
          </button>
        </div>
        <PublicKnowledgeCollectionPanel
          v-if="item.latest && item.latest.status === 'collected'"
          :key="item.latest.id"
          :collection="item.latest"
          @changed="load"
        />
        <PublicKnowledgeCollectionPanel
          v-else-if="item.published"
          :key="item.published.id"
          :collection="item.published"
          @changed="load"
        />
      </details>
    </template>
  </section>
</template>

<style scoped>
.public-knowledge {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  align-content: start;
  gap: 0.75rem;
  padding: 1rem 1.2rem 2rem;
}
.public-knowledge__steps {
  font-size: 0.85rem;
  color: rgb(var(--muted));
  padding-left: 18px;
  display: grid;
  gap: 2px;
}
.public-knowledge__toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 6px 0;
}
.public-knowledge__button {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
  font-size: 0.8rem;
  padding: 5px 12px;
  cursor: pointer;
}
.public-knowledge__button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.public-knowledge__status {
  font-size: 0.8rem;
  color: rgb(var(--muted));
  overflow-wrap: anywhere;
}
.public-knowledge__status--error {
  color: rgb(var(--danger));
}
.public-knowledge__source {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 10px 12px;
  background: rgb(var(--surface));
  min-width: 0;
}
.public-knowledge__source > summary {
  cursor: pointer;
  display: flex;
  flex-wrap: wrap;
  gap: 4px 10px;
  align-items: baseline;
}
.public-knowledge__title {
  font-weight: 600;
  font-size: 0.9rem;
}
.public-knowledge__badge {
  font-size: 0.75rem;
  color: rgb(var(--muted));
}
.public-knowledge__line {
  font-size: 0.8rem;
  color: rgb(var(--muted));
  margin-top: 4px;
  overflow-wrap: anywhere;
}
</style>
