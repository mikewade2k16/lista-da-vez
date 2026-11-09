<script setup lang="ts">
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AccountingIssuerForm from './AccountingIssuerForm.vue'
import AccountingDocumentFields from './AccountingDocumentFields.vue'
import AccountingKnowledge from './AccountingKnowledge.vue'
import AccountingObligations from './AccountingObligations.vue'
import AccountingQuestions from './AccountingQuestions.vue'
import { useAccountingWorkspace } from '../../composables/accounting/useAccountingWorkspace'
const workspace = useAccountingWorkspace()
const { state, accountId, dirty, enabled } = workspace
const states: Record<string, string> = {
  draft: 'Rascunho',
  validated: 'Validado localmente',
  reviewed: 'Revisado',
  authorized_for_submission: 'Envio autorizado',
  submitting: 'Processando',
  unknown: 'Resultado desconhecido',
  rejected: 'Rejeitado',
  authorized: 'Autorizado pelo conector',
}
function changeProfile(value: typeof state.profile) {
  state.profile = value
  state.profileDirty = true
  state.review = null
}
function changeIntent(value: string) {
  if (!['service', 'product', 'mixed'].includes(value)) return
  state.intent = value as typeof state.intent
  state.diagnosisDirty = true
  state.review = null
}
function changeCompetence(event: Event) {
  state.competenceDate = (event.target as HTMLInputElement).value
  state.diagnosisDirty = true
  state.review = null
}
function changeAnswers(value: typeof state.answers) {
  state.answers = value
  state.diagnosisDirty = true
  state.review = null
}
function changeFields(value: typeof state.fields) {
  state.fields = value
  state.documentDirty = true
  state.review = null
}
</script>
<template>
  <section class="accounting-workspace" aria-label="Diagnóstico empresarial">
    <p v-if="!enabled">O módulo Contábil não está disponível para esta conta ou usuário.</p>
    <template v-else>
      <p class="accounting-workspace__context">
        Conta: {{ accountId }} · Superfície: Finance · Dados empresariais
      </p>
      <p role="status">
        Formulários disponíveis sem IA. A conversa empresarial automática ainda está indisponível.
      </p>
      <AppSelectField
        label="Emitente autorizado"
        placeholder="Escolha explicitamente uma empresa"
        :model-value="state.issuer?.id || ''"
        :options="state.issuers.map((item) => ({ value: item.id, label: item.id }))"
        :disabled="state.busy"
        @update:model-value="workspace.selectIssuer"
      />
      <AppPanelButton
        v-if="state.more.issuers"
        variant="secondary"
        :disabled="state.busy"
        @click="workspace.loadMore('issuers')"
      >
        Carregar mais emitentes
      </AppPanelButton>
      <div class="accounting-workspace__actions">
        <AppPanelButton variant="secondary" :disabled="state.busy || dirty" @click="workspace.load">
          Atualizar lista
        </AppPanelButton>
        <AppPanelButton
          v-if="workspace.can('accounting.issuers.manage')"
          variant="secondary"
          :disabled="state.busy"
          @click="workspace.newIssuer"
        >
          Novo emitente
        </AppPanelButton>
      </div>
      <p v-if="state.error" role="alert">{{ state.error }}</p>
      <p v-if="state.message" role="status">{{ state.message }}</p>
      <p v-if="state.busy" role="status">Consultando o servidor...</p>
      <details :open="!state.issuer">
        <summary>Cadastro do emitente · origem declarada</summary>
        <AccountingIssuerForm
          :profile="state.profile"
          :dirty="state.profileDirty"
          :busy="state.busy"
          :writable="workspace.can('accounting.issuers.manage')"
          @change="changeProfile"
          @save="workspace.saveProfile"
        />
      </details>
      <template v-if="state.issuer">
        <AccountingKnowledge :key="state.issuer.id" :issuer-id="state.issuer.id" />
        <AccountingObligations
          :key="`obligations:${state.issuer.id}`"
          :issuer-id="state.issuer.id"
        />
        <p>{{ state.issuer.profile.legalName }} · Cadastro v{{ state.issuer.profileVersion }}</p>
        <details open>
          <summary>Diagnóstico e pendências</summary>
          <AppSelectField
            label="Retomar diagnóstico"
            :model-value="state.diagnosis?.id || ''"
            placeholder="Novo diagnóstico"
            :options="[
              { value: '', label: 'Novo diagnóstico' },
              ...state.diagnoses.map((item) => ({
                value: item.id,
                label: `${item.intent} · ${item.competenceDate || 'Competência desconhecida'} · v${item.version}`,
              })),
            ]"
            :disabled="state.busy"
            @update:model-value="workspace.selectDiagnosis"
          />
          <AppPanelButton
            v-if="state.more.diagnoses"
            variant="secondary"
            :disabled="state.busy"
            @click="workspace.loadMore('diagnoses')"
          >
            Carregar mais diagnósticos
          </AppPanelButton>
          <fieldset
            class="accounting-form__grid"
            :disabled="state.busy || !workspace.can('accounting.issuers.manage')"
          >
            <AppSelectField
              label="Operação"
              :model-value="state.intent"
              :options="[
                { value: 'service', label: 'Serviço' },
                { value: 'product', label: 'Produto' },
                { value: 'mixed', label: 'Mista' },
              ]"
              @update:model-value="changeIntent"
            />
            <label class="accounting-form__field">
              <span>Competência (não usar data do pagamento por suposição)</span>
              <input
                class="omni-input"
                type="date"
                :value="state.competenceDate"
                @input="changeCompetence"
              />
            </label>
          </fieldset>
          <AccountingQuestions
            :answers="state.answers"
            :intent="state.intent"
            :disabled="state.busy || !workspace.can('accounting.issuers.manage')"
            @change="changeAnswers"
          />
          <AppPanelButton
            v-if="workspace.can('accounting.issuers.manage')"
            :disabled="state.busy || state.profileDirty"
            @click="workspace.saveDiagnosis"
          >
            Salvar diagnóstico
          </AppPanelButton>
          <ul v-if="state.diagnosis?.pending.length">
            <li v-for="pending in state.diagnosis.pending" :key="pending.field">
              {{ pending.field }}: informação necessária para revisão e envio.
            </li>
          </ul>
          <p v-if="state.diagnosis?.nextQuestionId">
            Complete as pendências do cadastro e da operação abaixo. Informações desconhecidas
            continuam pendentes.
          </p>
        </details>
        <details v-if="workspace.can('accounting.documents.view')" open>
          <summary>Preparação e revisão de documentos</summary>
          <AppSelectField
            label="Rascunho existente"
            :model-value="state.document?.id || ''"
            placeholder="Escolha um rascunho"
            :options="
              state.documents.map((item) => ({
                value: item.id,
                label: `${item.kind.toUpperCase()} · ${states[item.state] || item.state} · ${item.id}`,
              }))
            "
            :disabled="state.busy"
            @update:model-value="workspace.selectDocument"
          />
          <AppPanelButton
            v-if="state.more.documents"
            variant="secondary"
            :disabled="state.busy"
            @click="workspace.loadMore('documents')"
          >
            Carregar mais rascunhos
          </AppPanelButton>
          <div
            v-if="workspace.can('accounting.documents.manage')"
            class="accounting-workspace__actions"
          >
            <AppPanelButton
              variant="secondary"
              :disabled="state.busy"
              @click="workspace.newDocument('nfse')"
            >
              Preparar serviço
            </AppPanelButton>
            <AppPanelButton
              variant="secondary"
              :disabled="state.busy"
              @click="workspace.newDocument('nfe')"
            >
              Preparar produto
            </AppPanelButton>
          </div>
          <template v-if="state.documentEditorOpen">
            <p>
              Tipo selecionado:
              {{ state.kind === 'nfse' ? 'NFS-e de serviço' : 'NF-e de produto' }}.
            </p>
            <p v-if="state.document">
              {{ states[state.document.state] || state.document.state }} · Versão
              {{ state.document.version }}
            </p>
            <AccountingDocumentFields
              :fields="state.fields"
              :disabled="
                state.busy ||
                !workspace.can('accounting.documents.manage') ||
                Boolean(
                  state.document &&
                  !['draft', 'validated', 'reviewed', 'rejected'].includes(state.document.state),
                )
              "
              @change="changeFields"
            />
            <p v-if="state.documentDirty" role="status">
              Rascunho alterado; salve e refaça a revisão.
            </p>
            <div class="accounting-workspace__actions">
              <AppPanelButton
                v-if="workspace.can('accounting.documents.manage')"
                :disabled="
                  state.busy || !state.diagnosis || state.profileDirty || state.diagnosisDirty
                "
                @click="workspace.saveDocument"
              >
                Salvar rascunho
              </AppPanelButton>
              <AppPanelButton
                v-if="
                  workspace.can('accounting.documents.manage') && state.document?.state === 'draft'
                "
                variant="secondary"
                :disabled="state.busy || dirty"
                @click="workspace.validate"
              >
                Validar dados locais
              </AppPanelButton>
              <AppPanelButton
                variant="secondary"
                :disabled="state.busy || dirty || !state.document"
                @click="workspace.loadReview"
              >
                Conferir revisão
              </AppPanelButton>
            </div>
            <div v-if="state.review" class="accounting-workspace__review">
              <h4>Snapshot do servidor para revisão</h4>
              <p>
                Emitente: {{ state.review.profile.legalName }} · Competência:
                {{ state.review.document.fields.competenceDate || 'Desconhecida' }} · Valor:
                {{ state.review.document.fields.amount || 'Desconhecido' }}
              </p>
              <ul>
                <li v-for="field in state.review.pending" :key="field">Pendente: {{ field }}</li>
              </ul>
              <p>Origem: cadastro e rascunho persistidos. Revisar não autoriza envio.</p>
              <AppPanelButton
                v-if="
                  workspace.can('accounting.documents.review') &&
                  state.document?.state === 'validated'
                "
                :disabled="state.busy || dirty || Boolean(state.review.pending.length)"
                @click="workspace.confirmReview"
              >
                Confirmar revisão dos dados
              </AppPanelButton>
              <p v-if="!state.review.emissionAvailable" role="status">
                Emissão indisponível para este cenário. Nenhuma nota foi emitida.
              </p>
            </div>
          </template>
        </details>
      </template>
    </template>
  </section>
</template>
<style scoped>
.accounting-workspace {
  display: grid;
  gap: 0.75rem;
  padding: 0.75rem;
  min-width: 0;
  overflow-wrap: anywhere;
}
.accounting-workspace__context {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}
.accounting-workspace__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.accounting-workspace details {
  border: 1px solid rgb(var(--border));
  border-radius: 0.5rem;
  padding: 0.625rem;
}
.accounting-workspace summary {
  cursor: pointer;
  font-weight: 600;
  margin-bottom: 0.5rem;
}
.accounting-workspace__review {
  margin-top: 0.75rem;
}
.accounting-workspace :deep(.accounting-form) {
  display: grid;
  gap: 0.5rem;
}
.accounting-workspace :deep(.accounting-form__grid) {
  display: grid;
  gap: 0.5rem;
  border: 0;
  padding: 0;
  min-width: 0;
}
.accounting-workspace :deep(.accounting-form__field) {
  display: grid;
  gap: 0.25rem;
  min-width: 0;
}
@media (min-width: 600px) {
  .accounting-workspace :deep(.accounting-form__grid) {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
