<script setup lang="ts">
import { ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import KnowledgeMetadataForm from './KnowledgeMetadataForm.vue'
import KnowledgeIngestion from './KnowledgeIngestion.vue'
import { useAccountingKnowledge } from '../../composables/accounting/useAccountingKnowledge'
import { emptyKnowledgeMetadata, type KnowledgeDocument } from '../../types/knowledge'
import { confirmFiscalAction } from '../../utils/fiscal-confirm'
const props = defineProps<{ issuerId: string }>()
const workspace = useAccountingKnowledge(() => props.issuerId),
  { state } = workspace
const baseName = ref(''),
  title = ref(''),
  sourceRef = ref(''),
  content = ref(''),
  file = ref<File | null>(null)
const metadata = ref(emptyKnowledgeMetadata()),
  selected = ref<KnowledgeDocument | null>(null),
  reviewMetadata = ref(emptyKnowledgeMetadata())
const query = ref(''),
  competence = ref(''),
  jurisdiction = ref(''),
  regime = ref(''),
  fileBusy = ref(false)
watch(workspace.scope, () => {
  title.value = ''
  sourceRef.value = ''
  content.value = ''
  file.value = null
  selected.value = null
  metadata.value = emptyKnowledgeMetadata()
  state.result = null
})
function fileChanged(event: Event) {
  file.value = (event.target as HTMLInputElement).files?.[0] || null
}
async function ingest() {
  const scope = workspace.scope.value,
    base = state.baseId
  if (fileBusy.value) return
  fileBusy.value = true
  try {
    const original = file.value
    if (original && original.size > 2 * 1024 * 1024) {
      state.error = 'O arquivo deve ter até 2 MiB.'
      return
    }
    const bytes = original
      ? new Uint8Array(await original.arrayBuffer())
      : new TextEncoder().encode(content.value)
    if (bytes.length > 2 * 1024 * 1024) {
      state.error = 'O texto deve ter até 2 MiB.'
      return
    }
    let binary = ''
    for (let start = 0; start < bytes.length; start += 32768)
      binary += String.fromCharCode(...bytes.subarray(start, start + 32768))
    if (scope !== workspace.scope.value || base !== state.baseId) return
    const ok = await workspace.ingest({
      title: title.value,
      sourceRef: sourceRef.value,
      fileName: original?.name || 'conteudo-selecionado.txt',
      contentType: original
        ? original.name.toLowerCase().endsWith('.pdf')
          ? 'application/pdf'
          : 'text/plain'
        : 'text/plain',
      content: btoa(binary),
      metadata: JSON.parse(JSON.stringify(metadata.value)),
    })
    if (ok && scope === workspace.scope.value) {
      content.value = ''
      file.value = null
    }
  } catch {
    if (scope === workspace.scope.value) state.error = 'Não foi possível ler o arquivo selecionado.'
  } finally {
    fileBusy.value = false
  }
}
function select(doc: KnowledgeDocument) {
  selected.value = doc
  reviewMetadata.value = JSON.parse(JSON.stringify(doc.metadata))
}
async function review() {
  if (!selected.value) return
  if (await workspace.review(selected.value, reviewMetadata.value)) selected.value = null
}
async function publish(doc: KnowledgeDocument) {
  if (
    await confirmFiscalAction(
      'Publicar esta versão com os metadados revisados? O texto publicado será imutável.',
      () => `${workspace.scope.value}:${doc.id}:${doc.lockVersion}`,
    )
  )
    await workspace.publish(doc)
}
</script>
<template>
  <details v-if="workspace.allowed()" class="accounting-knowledge">
    <summary>Fontes e conhecimento fiscal · {{ state.bases.length }} bases carregadas</summary>
    <KnowledgeIngestion v-if="state.baseId" :issuer-id="props.issuerId" :base-id="state.baseId" />
    <p>
      Fontes privadas deste emitente. Busca textual sem IA; score não é confiança jurídica. Vigência
      desconhecida permanece informativa.
    </p>
    <p v-if="state.error" role="alert">{{ state.error }}</p>
    <AppPanelButton :disabled="state.busy" @click="workspace.load">
      Carregar bases autorizadas
    </AppPanelButton>
    <AppSelectField
      label="Base deste emitente"
      :model-value="state.baseId"
      :disabled="state.busy"
      :options="state.bases.map((item) => ({ value: item.id, label: item.name }))"
      @update:model-value="workspace.choose"
    />
    <details v-if="workspace.allowed(true)">
      <summary>Gerenciar bases · desativadas por padrão</summary>
      <form @submit.prevent="workspace.createBase(baseName)">
        <label>
          Nome
          <input v-model="baseName" class="omni-input" maxlength="200" />
        </label>
        <AppPanelButton type="submit" :disabled="state.busy || !baseName.trim()">
          Criar base privada
        </AppPanelButton>
      </form>
      <div v-for="base in state.bases" :key="base.id">
        <span>{{ base.name }} · {{ base.enabled ? 'Ativa' : 'Desativada' }}</span>
        <AppPanelButton variant="secondary" :disabled="state.busy" @click="workspace.toggle(base)">
          {{ base.enabled ? 'Desativar' : 'Ativar' }}
        </AppPanelButton>
      </div>
    </details>
    <details v-if="workspace.allowed(true) && state.baseId">
      <summary>Importar conteúdo selecionado / texto / PDF</summary>
      <form class="accounting-knowledge__form" @submit.prevent="ingest">
        <label>
          Título
          <input v-model="title" class="omni-input" maxlength="500" />
        </label>
        <label>
          Origem verificável (https:// ou manual://)
          <input v-model="sourceRef" class="omni-input" maxlength="1000" />
        </label>
        <KnowledgeMetadataForm v-model="metadata" />
        <label>
          Texto selecionado
          <textarea v-model="content" class="omni-input" rows="4"></textarea>
        </label>
        <label>
          Ou arquivo original (até 2 MiB; PDF textual até 10 páginas)
          <input type="file" accept=".txt,.pdf,text/plain,application/pdf" @change="fileChanged" />
        </label>
        <p>
          Nenhuma URL é baixada automaticamente. O original será guardado no storage privado. PDF
          digitalizado exige transcrição manual; extração parcial falha sem publicar.
        </p>
        <AppPanelButton
          type="submit"
          :disabled="
            state.busy ||
            fileBusy ||
            !title.trim() ||
            !sourceRef.trim() ||
            (!content.trim() && !file)
          "
        >
          Importar como rascunho
        </AppPanelButton>
      </form>
    </details>
    <details v-if="state.baseId">
      <summary>Versões importadas · {{ state.documents.length }} carregadas</summary>
      <article v-for="doc in state.documents" :key="doc.id">
        <strong>{{ doc.title }} · v{{ doc.version }} · {{ doc.status }}</strong>
        <p>{{ doc.sourceRef }} · {{ doc.chunkCount }} trechos</p>
        <template v-if="workspace.allowed(true)">
          <AppPanelButton
            v-if="doc.status === 'draft'"
            variant="secondary"
            :disabled="state.busy"
            @click="select(doc)"
          >
            Revisar metadados
          </AppPanelButton>
          <AppPanelButton
            v-if="doc.status === 'draft' && doc.reviewedBy"
            :disabled="state.busy"
            @click="publish(doc)"
          >
            Publicar versão revisada
          </AppPanelButton>
          <AppPanelButton
            v-if="doc.status === 'published'"
            variant="secondary"
            :disabled="state.busy"
            @click="workspace.deactivate(doc)"
          >
            Desativar versão
          </AppPanelButton>
        </template>
      </article>
      <AppPanelButton
        v-if="state.more"
        variant="secondary"
        :disabled="state.busy"
        @click="workspace.next"
      >
        Mais versões
      </AppPanelButton>
      <form v-if="selected" class="accounting-knowledge__form" @submit.prevent="review">
        <KnowledgeMetadataForm v-model="reviewMetadata" />
        <p>
          Confirme somente metadados conferidos. Revisar não atesta enquadramento tributário do
          emitente.
        </p>
        <AppPanelButton type="submit" :disabled="state.busy">
          Salvar revisão explícita
        </AppPanelButton>
      </form>
    </details>
    <details>
      <summary>Consultar fontes publicadas</summary>
      <form
        class="accounting-knowledge__form"
        @submit.prevent="workspace.search(query, competence, jurisdiction, regime)"
      >
        <label>
          Pergunta / termos
          <input v-model="query" class="omni-input" maxlength="500" />
        </label>
        <label>
          Competência
          <input v-model="competence" type="date" class="omni-input" />
        </label>
        <label>
          Jurisdição para filtrar
          <input v-model="jurisdiction" class="omni-input" maxlength="100" />
        </label>
        <label>
          Regime para filtrar
          <input v-model="regime" class="omni-input" maxlength="100" />
        </label>
        <AppPanelButton type="submit" :disabled="state.busy || !query.trim()">
          Buscar sem IA
        </AppPanelButton>
      </form>
      <template v-if="state.result">
        <p>{{ state.result.message }}</p>
        <p v-if="state.result.truncated">
          Resultado truncado pelo limite de saída. Refine a consulta.
        </p>
        <p v-if="state.result.potentialConflict">
          Múltiplas fontes: revisar possível conflito; ranking não resolve divergência normativa.
        </p>
        <p v-if="!state.result.evidence.length">Sem evidência no corpus autorizado.</p>
        <article v-for="evidence in state.result.evidence" :key="evidence.chunkId">
          <blockquote>{{ evidence.excerpt }}</blockquote>
          <p>
            {{ evidence.sourceRef }} · v{{ evidence.version }} · offsets
            {{ evidence.location.start }}–{{ evidence.location.end }}
          </p>
          <small>
            {{ evidence.documentId }} / {{ evidence.chunkId }} ·
            {{
              evidence.applicable
                ? 'Contexto confirmado'
                : 'Informativa / aplicabilidade não confirmada'
            }}
          </small>
        </article>
      </template>
    </details>
  </details>
</template>
<style scoped>
.accounting-knowledge {
  display: grid;
  gap: 0.5rem;
  min-width: 0;
  overflow-wrap: anywhere;
}
.accounting-knowledge__form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.5rem;
}
.accounting-knowledge__form label {
  display: grid;
  gap: 0.25rem;
}
.accounting-knowledge article {
  padding: 0.5rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
}
.accounting-knowledge blockquote {
  white-space: pre-wrap;
}
</style>
