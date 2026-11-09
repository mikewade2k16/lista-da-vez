<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { usePublicKnowledgeStore } from '~/stores/public-knowledge'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import {
  canPublish,
  formatPublicDate,
  reviewSummaryLabel,
  verdictLabel,
} from '~/utils/public-knowledge'
import type {
  PublicChunk,
  PublicCollection,
  PublicReview,
  PublicReviewInput,
} from '~/types/public-knowledge'

// Revisão de uma coleta: trechos (excluir os ruins), revisões registradas (você,
// outras IAs, contador), exportação do texto para revisão externa e publicação.
const props = defineProps<{ collection: PublicCollection }>()
const emit = defineEmits<{ changed: [] }>()
const store = usePublicKnowledgeStore()
const ui = useUiStore()
const chunks = ref<PublicChunk[]>([])
const page = ref(1)
const more = ref(false)
const reviews = ref<PublicReview[]>([])
const busy = ref(false)
const error = ref('')
const status = ref('')
const form = reactive<PublicReviewInput>({
  reviewerKind: 'owner',
  reviewerName: 'Eu (dono)',
  verdict: 'approved',
  notes: '',
})
const editable = props.collection.status === 'collected'

async function load(next = 1) {
  error.value = ''
  try {
    const list = await store.chunks(props.collection.id, next)
    chunks.value = next === 1 ? list : [...chunks.value, ...list]
    page.value = next
    more.value = list.length === 50
    if (next === 1) reviews.value = await store.reviews(props.collection.id)
  } catch (cause) {
    error.value = getApiErrorMessage(cause, 'Não foi possível carregar a coleta.')
  }
}
async function run(action: () => Promise<unknown>, done: string, refresh = true) {
  busy.value = true
  error.value = ''
  status.value = ''
  try {
    await action()
    status.value = done
    if (refresh) emit('changed')
  } catch (cause) {
    error.value = getApiErrorMessage(cause, 'Não foi possível concluir a ação.')
  } finally {
    busy.value = false
  }
}
async function confirm(message: string) {
  return ((await ui.confirm({ title: 'Base pública', message })) as { confirmed: boolean })
    .confirmed
}
async function exclude(chunk: PublicChunk) {
  await run(
    () => store.excludeChunk(props.collection.id, chunk.id),
    `Trecho ${chunk.index + 1} excluído desta versão.`,
    false,
  )
  chunk.status = 'withdrawn'
}
async function saveReview() {
  await run(() => store.review(props.collection.id, { ...form }), 'Revisão registrada.')
  form.notes = ''
  reviews.value = await store.reviews(props.collection.id)
}
async function publish() {
  if (
    await confirm(
      'Publicar esta versão? Os trechos passam a aparecer no chat de todas as contas e a versão anterior desta fonte deixa de aparecer.',
    )
  )
    await run(() => store.publish(props.collection.id), 'Publicada.')
}
async function discard() {
  if (
    await confirm(
      'Descartar esta coleta? Os trechos não serão publicados. O histórico continua registrado.',
    )
  )
    await run(() => store.discard(props.collection.id), 'Coleta descartada.')
}
async function exportText() {
  await run(
    async () => {
      const blob = await store.exportText(props.collection.id)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${props.collection.catalogKey}-v${props.collection.version}.md`
      link.click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    },
    'Texto exportado. Envie às outras IAs ou ao contador e registre o resultado abaixo.',
    false,
  )
}
onMounted(() => void load())
</script>

<template>
  <div class="public-collection">
    <p class="public-collection__meta">
      Versão {{ collection.version }} · {{ collection.chunkCount }} trechos
      <template v-if="collection.excluded">({{ collection.excluded }} excluídos)</template>
      <template v-if="collection.pages">· {{ collection.pages }} páginas</template>
      · coletada em {{ formatPublicDate(collection.collectedAt) }} ·
      <a :href="collection.sourceRef" target="_blank" rel="noopener noreferrer">abrir original</a>
    </p>
    <p class="public-collection__meta">{{ reviewSummaryLabel(collection.review) }}</p>
    <p v-if="error" class="public-collection__status public-collection__status--error" role="alert">
      {{ error }}
    </p>
    <p v-if="status" class="public-collection__status" role="status">{{ status }}</p>

    <div class="public-collection__actions">
      <button type="button" class="public-collection__button" :disabled="busy" @click="exportText">
        Exportar texto para revisão
      </button>
      <button
        v-if="editable"
        type="button"
        class="public-collection__button public-collection__button--primary"
        :disabled="busy || !canPublish(collection.review)"
        :title="
          canPublish(collection.review) ? '' : 'Registre sua aprovação (dono) antes de publicar'
        "
        @click="publish"
      >
        Publicar
      </button>
      <button
        v-if="editable"
        type="button"
        class="public-collection__button public-collection__button--danger"
        :disabled="busy"
        @click="discard"
      >
        Descartar
      </button>
    </div>

    <details class="public-collection__section">
      <summary>Revisões ({{ reviews.length }})</summary>
      <form class="public-collection__form" @submit.prevent="saveReview">
        <label>
          Quem revisou
          <select v-model="form.reviewerKind">
            <option value="owner">Eu (dono)</option>
            <option value="ai">Outra IA</option>
            <option value="accountant">Contador</option>
          </select>
        </label>
        <label>
          Nome
          <input
            v-model.trim="form.reviewerName"
            maxlength="120"
            required
            placeholder="Ex.: ChatGPT, Gemini, Contador Fulano"
          />
        </label>
        <label>
          Resultado
          <select v-model="form.verdict">
            <option value="approved">Aprovado</option>
            <option value="changes_requested">Pediu ajustes</option>
            <option value="rejected">Reprovado</option>
          </select>
        </label>
        <label class="public-collection__wide">
          Observações
          <textarea v-model="form.notes" maxlength="4000" rows="2"></textarea>
        </label>
        <button type="submit" class="public-collection__button" :disabled="busy">
          Registrar revisão
        </button>
      </form>
      <ul class="public-collection__reviews">
        <li v-for="review in reviews" :key="review.id">
          {{ formatPublicDate(review.createdAt) }} · {{ review.reviewerName }} ({{
            review.reviewerKind === 'owner'
              ? 'dono'
              : review.reviewerKind === 'ai'
                ? 'IA'
                : 'contador'
          }}): {{ verdictLabel(review.verdict) }}
          <template v-if="review.notes">— {{ review.notes }}</template>
        </li>
      </ul>
    </details>

    <details class="public-collection__section">
      <summary>Trechos</summary>
      <ol class="public-collection__chunks">
        <li
          v-for="chunk in chunks"
          :key="chunk.id"
          class="public-collection__chunk"
          :class="{ 'public-collection__chunk--excluded': chunk.status === 'withdrawn' }"
        >
          <div class="public-collection__chunk-head">
            <strong>{{ chunk.index + 1 }}.</strong>
            <span>
              {{ chunk.heading || 'sem título' }}
              <template v-if="chunk.page">· p. {{ chunk.page }}</template>
            </span>
            <span v-if="chunk.status === 'withdrawn'">excluído</span>
            <button
              v-else-if="editable"
              type="button"
              class="public-collection__button public-collection__button--danger"
              :disabled="busy"
              @click="exclude(chunk)"
            >
              Excluir trecho
            </button>
          </div>
          <p class="public-collection__chunk-text">{{ chunk.text }}</p>
        </li>
      </ol>
      <button v-if="more" type="button" class="public-collection__button" @click="load(page + 1)">
        Carregar mais trechos
      </button>
    </details>
  </div>
</template>

<style scoped>
.public-collection {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.public-collection__meta,
.public-collection__status {
  font-size: 0.8rem;
  color: rgb(var(--muted));
  overflow-wrap: anywhere;
}
.public-collection__status--error {
  color: rgb(var(--danger));
}
.public-collection__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.public-collection__button {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
  font-size: 0.78rem;
  padding: 4px 10px;
  cursor: pointer;
}
.public-collection__button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.public-collection__button--primary {
  border-color: rgb(var(--primary));
  color: rgb(var(--primary));
}
.public-collection__button--danger {
  color: rgb(var(--danger));
}
.public-collection__section {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 8px 10px;
}
.public-collection__section > summary {
  cursor: pointer;
  font-weight: 600;
  font-size: 0.85rem;
}
.public-collection__form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 8px;
  margin-top: 8px;
  align-items: end;
}
.public-collection__form label {
  display: grid;
  gap: 4px;
  font-size: 0.75rem;
}
.public-collection__form input,
.public-collection__form select,
.public-collection__form textarea {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 6px 8px;
  color: rgb(var(--text));
  background: rgb(var(--surface));
}
.public-collection__wide {
  grid-column: 1 / -1;
}
.public-collection__reviews {
  margin-top: 8px;
  padding-left: 16px;
  font-size: 0.78rem;
  color: rgb(var(--muted));
}
.public-collection__chunks {
  display: grid;
  gap: 6px;
  margin-top: 8px;
  padding: 0;
  list-style: none;
}
.public-collection__chunk {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 6px 8px;
  background: rgb(var(--surface));
}
.public-collection__chunk--excluded {
  opacity: 0.5;
}
.public-collection__chunk-head {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 8px;
  align-items: center;
  font-size: 0.75rem;
  color: rgb(var(--muted));
}
.public-collection__chunk-text {
  font-size: 0.8rem;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  margin-top: 4px;
}
</style>
