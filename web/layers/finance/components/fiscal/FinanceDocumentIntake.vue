<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import FinanceServiceFields from './FinanceServiceFields.vue'
import FinanceDocumentReview from './FinanceDocumentReview.vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useFinanceDocuments } from '../../composables/useFinanceDocuments'
const props = defineProps<{ expanded?: boolean; documentId?: string; reviewExpanded?: boolean }>()
const emit = defineEmits<{
  discuss: [question: string]
  selected: [id: string]
  dirty: [value: boolean]
}>()
const editor = useFinanceDocuments(() => props.documentId || '')
const prepared = ref(false)
const { documents, templates, current, fields, busy, error, status, file, dirty } = editor
const fileInput = ref<HTMLInputElement | null>(null)
watch(
  () => current.value?.id,
  (id) => {
    if (id) emit('selected', id)
  },
)
watch(dirty, (value) => emit('dirty', value), { immediate: true })
async function prepareReview() {
  prepared.value = await editor.prepare()
}

watch(
  [file, fileInput] as const,
  ([selected, input]) => {
    if (!selected && input) input.value = ''
  },
  { immediate: true, flush: 'post' },
)
const options = computed(() => documents.value.map((d) => ({ value: d.id, label: d.sourceName })))
const templateOptions = computed(() => [
  { value: '', label: 'Sem template' },
  ...templates.value.map((t) => ({ value: t.id, label: t.name })),
])
onBeforeUnmount(() => emit('dirty', false))
function chooseFile(event: Event) {
  file.value = (event.target as HTMLInputElement).files?.[0] || null
}
function chooseDocument(id: string) {
  const doc = documents.value.find((d) => d.id === id)
  if (doc && !dirty.value) editor.select(doc)
}
function discuss() {
  if (!current.value || dirty.value) return
  emit(
    'discuss',
    `Ajude a preparar a NFS-e a partir deste comprovante. Rascunho ${current.value.id}. Dados para revisar: ${JSON.stringify(current.value.fields).slice(0, 3000)}. Confira o emitente no meu perfil e pergunte o que falta. Não emita a nota.`,
  )
}
</script>
<template>
  <details class="finance-document" :open="expanded">
    <summary>Enviar comprovante e preparar nota</summary>
    <div class="finance-document__body">
      <div class="finance-document__upload">
        <label class="finance-document__field">
          Comprovante · até 8 MB
          <input ref="fileInput" type="file" :disabled="busy || dirty" @change="chooseFile" />
        </label>
        <AppPanelButton :disabled="busy || !file || dirty" @click="editor.extract">
          {{ busy ? 'Processando...' : 'Ler documento' }}
        </AppPanelButton>
        <NuxtLink to="/perfil#fiscal">Perfil fiscal</NuxtLink>
      </div>
      <p v-if="error" role="alert">{{ error }}</p>
      <p v-if="status" role="status">{{ status }}</p>
      <AppSelectField
        v-if="documents.length"
        :model-value="current?.id || ''"
        label="Rascunhos salvos"
        :options="options"
        :disabled="busy || dirty"
        @update:model-value="chooseDocument(String($event))"
      />
      <form v-if="current && fields" @submit.prevent="editor.save">
        <fieldset :disabled="busy" class="finance-document__fields">
          <div class="finance-document__toolbar">
            <span class="finance-document__notice">Confira os dados antes de continuar.</span>
            <AppPanelButton
              variant="ghost"
              :disabled="busy || dirty"
              title="Completar nomes e CPF/CNPJ vazios a partir do texto salvo"
              @click="editor.reidentifyParties"
            >
              Atualizar leitura
            </AppPanelButton>
          </div>
          <div class="finance-document__grid">
            <div
              v-for="party in ['payer', 'payee'] as const"
              :key="party"
              class="finance-document__party"
            >
              <div class="finance-document__toolbar">
                <strong>{{ party === 'payer' ? 'Pagador' : 'Recebedor' }}</strong>
                <AppPanelButton
                  variant="ghost"
                  :disabled="!fields[party === 'payer' ? 'payerName' : 'payeeName']"
                  @click="editor.usePartyAsCustomer(party)"
                >
                  Usar como tomador
                </AppPanelButton>
              </div>
              <label class="finance-document__field">
                Nome
                <input
                  v-model="fields[party === 'payer' ? 'payerName' : 'payeeName']"
                  maxlength="600"
                />
              </label>
              <label class="finance-document__field">
                CPF/CNPJ
                <input
                  v-model="fields[party === 'payer' ? 'payerDocument' : 'payeeDocument']"
                  maxlength="32"
                  placeholder="Não identificado ou oculto"
                />
              </label>
            </div>
          </div>
          <div class="finance-document__grid">
            <label class="finance-document__field">
              Valor do serviço (R$)
              <input v-model.number="fields.amount" type="number" min="0" step="0.01" />
            </label>
            <label class="finance-document__field">
              Data do pagamento
              <input v-model="fields.paymentDate" type="date" />
            </label>
            <label class="finance-document__field">
              Tomador do serviço
              <input v-model="fields.customerName" maxlength="300" />
            </label>
            <label class="finance-document__field">
              CPF/CNPJ do tomador
              <input v-model="fields.customerDocument" maxlength="32" />
            </label>
            <label class="finance-document__field">
              Competência do serviço
              <input v-model="fields.competenceDate" type="date" />
            </label>
          </div>
          <AppSelectField
            :model-value="fields.templateId"
            label="Template de serviço"
            :options="templateOptions"
            @update:model-value="editor.applyTemplate(String($event))"
          />
          <label class="finance-document__field">
            Descrição do serviço
            <textarea
              v-model="fields.service.serviceDescription"
              maxlength="4000"
              rows="2"
            ></textarea>
          </label>
          <details class="finance-document__additional">
            <summary>Tributação e observações</summary>
            <div class="finance-document__body">
              <FinanceServiceFields :service="fields.service" />
              <label class="finance-document__field">
                Observações
                <textarea v-model="fields.service.notes" maxlength="2000" rows="2"></textarea>
              </label>
            </div>
          </details>
          <p v-if="current.missingFields.length">
            Falta completar no rascunho salvo: {{ current.missingFields.join(', ') }}.
          </p>
          <p v-if="dirty" role="status">
            Alterações não salvas. Salve antes de trocar de documento ou consultar a IA.
          </p>
          <div class="finance-document__actions">
            <AppPanelButton type="submit" :disabled="busy || !dirty">
              Salvar rascunho
            </AppPanelButton>
            <AppPanelButton :disabled="busy || dirty" variant="secondary" @click="prepareReview">
              Preparar revisão
            </AppPanelButton>
            <AppPanelButton :disabled="busy || dirty" variant="secondary" @click="discuss">
              Consultar IA sobre este rascunho
            </AppPanelButton>
          </div>
        </fieldset>
        <FinanceDocumentReview
          :id="current.id"
          :version="current.updatedAt"
          :dirty="dirty"
          :expanded="reviewExpanded || prepared"
        />
        <details>
          <summary>Texto lido do documento</summary>
          <pre class="finance-document__text">{{ current.extractedText }}</pre>
        </details>
      </form>
      <details class="finance-document__help">
        <summary>Formatos e orientações</summary>
        <p>
          PDF (até 10 páginas), JPEG, PNG, TXT, CSV ou OFX. Desbloqueie arquivos protegidos por
          senha.
        </p>
        <p v-for="warning in current?.warnings || []" :key="warning">{{ warning }}</p>
      </details>
      <p class="finance-document__notice">
        Rascunho para revisão · emissão no portal ainda indisponível.
      </p>
    </div>
  </details>
</template>
<style scoped>
.finance-document {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 10px;
  margin-bottom: 12px;
}
.finance-document summary {
  cursor: pointer;
  font-weight: 600;
}
.finance-document__body {
  display: grid;
  gap: 10px;
  padding-top: 10px;
}
.finance-document__body p {
  font-size: 0.8rem;
  margin: 0;
}
.finance-document__fields {
  display: grid;
  gap: 10px;
  border: 0;
  padding: 0;
  min-width: 0;
}
.finance-document__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px;
}
.finance-document__field {
  display: grid;
  gap: 5px;
  font-size: 0.8rem;
}
.finance-document__field input,
.finance-document__field textarea {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 7px;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: rgb(var(--text));
}
.finance-document__notice {
  color: rgb(var(--muted));
}
.finance-document__upload,
.finance-document__toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.finance-document__upload > label {
  flex: 1 1 220px;
  min-width: 0;
}
.finance-document__toolbar {
  justify-content: space-between;
  font-size: 0.8rem;
}
.finance-document__party {
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(0, 2fr);
  gap: 6px;
  min-width: 0;
  padding: 8px;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
}
.finance-document__party > .finance-document__toolbar {
  grid-column: 1 / -1;
}
@media (max-width: 480px) {
  .finance-document__party {
    grid-template-columns: minmax(0, 1fr);
  }
}
.finance-document__help {
  font-size: 0.75rem;
  color: rgb(var(--muted));
}
.finance-document__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.finance-document__text {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font-size: 0.75rem;
}
</style>
