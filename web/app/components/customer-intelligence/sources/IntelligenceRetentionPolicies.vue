<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useRetentionPolicies } from '~/composables/customer-intelligence/useRetentionPolicies'
import {
  RETENTION_EXPIRY_OPTIONS,
  validRetentionDraftCommand,
  validRetentionPublishCommand,
  type RetentionExpiryAction,
  type RetentionPolicyDraftCommand,
  type RetentionPolicyPublishCommand,
  type RetentionPublicationReasonCode,
} from '~/domain/customer-intelligence/retention-policy-types'
import { useUiStore } from '~/stores/ui'
import { isRetentionPublishConfirmed } from './retention-publish-confirmation'

const DAY_SECONDS = 86_400

const retention = useRetentionPolicies()
const ui = useUiStore()
const draftPolicyKey = ref('customer_profile.default')
const draftDays = ref(90)
const draftExpiry = ref<RetentionExpiryAction>('tombstone')
const publicationReason = ref<RetentionPublicationReasonCode>('legal_review_approved')
const approvalReference = ref('')

const draftCommand = computed<RetentionPolicyDraftCommand>(() => ({
  policyKey: draftPolicyKey.value.trim(),
  snapshotTtlSeconds: Number(draftDays.value) * DAY_SECONDS,
  onExpiry: draftExpiry.value,
}))
const publishCommand = computed<RetentionPolicyPublishCommand | null>(() => {
  if (!retention.selectedDraft.value) return null
  return {
    expectedRevision: retention.selectedDraft.value.revision,
    reasonCode: publicationReason.value,
    approvalReference: approvalReference.value.trim(),
  }
})
const canCreateDraft = computed(
  () =>
    retention.access.canManageSources.value &&
    !retention.savingAction.value &&
    validRetentionDraftCommand(draftCommand.value),
)
const canPublish = computed(
  () =>
    retention.access.canManageSources.value &&
    !retention.savingAction.value &&
    Boolean(publishCommand.value && validRetentionPublishCommand(publishCommand.value)),
)
const policyOptions = computed(() =>
  retention.policyKeys.value.map((policyKey) => ({
    value: policyKey,
    label: policyKey,
  })),
)
const draftOptions = computed(() =>
  retention.selectedDrafts.value.map((version) => ({
    value: version.id,
    label: `v${version.version} · revisao ${version.revision}`,
    meta: `${formatDays(version.snapshotTtlSeconds)} · ${expiryLabel(version.onExpiry)}`,
  })),
)

watch(
  [retention.scopeKey, retention.selectedPolicyKey],
  () => {
    const baseline = retention.selectedVersions.value[0]
    draftPolicyKey.value = retention.selectedPolicyKey.value || 'customer_profile.default'
    draftDays.value = baseline
      ? Math.max(1, Math.round(baseline.snapshotTtlSeconds / DAY_SECONDS))
      : 90
    draftExpiry.value = baseline?.onExpiry ?? 'tombstone'
    publicationReason.value = 'legal_review_approved'
    approvalReference.value = ''
  },
  { immediate: true },
)

function formatDays(seconds: number): string {
  const days = Math.max(1, Math.round(Number(seconds || 0) / DAY_SECONDS))
  return `${days} ${days === 1 ? 'dia' : 'dias'}`
}

function expiryLabel(action: RetentionExpiryAction): string {
  return RETENTION_EXPIRY_OPTIONS.find((option) => option.value === action)?.label ?? action
}

function formatDate(value?: string): string {
  if (!value) return '—'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '—'
  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(parsed)
}

async function createDraft(): Promise<void> {
  if (!canCreateDraft.value) return
  const created = await retention.createDraft(draftCommand.value)
  if (created) {
    ui.success(
      `Uma nova versao draft de ${draftCommand.value.policyKey} foi criada. Ela ainda nao esta publicada.`,
      'Draft criado',
    )
  }
}

async function publishDraft(): Promise<void> {
  const version = retention.selectedDraft.value
  const command = publishCommand.value
  if (!version || !command || !canPublish.value) return
  const confirmation = await ui.confirm({
    title: 'Publicar retention policy?',
    message:
      `A versao ${version.version} de ${version.policyKey} sera publicada com revisao otimista ` +
      `${version.revision}. Fontes existentes nao serao repontadas automaticamente.`,
    confirmLabel: 'Publicar versao',
    cancelLabel: 'Manter como draft',
  })
  if (!isRetentionPublishConfirmed(confirmation)) return
  const published = await retention.publishDraft(version, command)
  if (published) {
    approvalReference.value = ''
    ui.success(
      `A versao ${version.version} de ${version.policyKey} foi publicada.`,
      'Policy publicada',
    )
  }
}
</script>

<template>
  <section class="retention-policies" aria-labelledby="retention-policies-title">
    <header class="retention-policies__header">
      <div>
        <small>Governanca de dados</small>
        <h2 id="retention-policies-title">Policies de retencao</h2>
        <p>
          Versoes sao imutaveis depois da publicacao. Criar um draft nunca publica nem altera o
          binding das fontes existentes.
        </p>
      </div>
      <button
        type="button"
        :disabled="retention.loading.value || Boolean(retention.savingAction.value)"
        @click="retention.load()"
      >
        {{ retention.loading.value ? 'Atualizando...' : 'Atualizar' }}
      </button>
    </header>

    <CustomerIntelligenceStatus
      v-if="retention.loading.value && !retention.policies.value.length"
      title="Carregando policies de retencao"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="retention.error.value && !retention.policies.value.length"
      title="Policies de retencao indisponiveis"
      :error="retention.error.value"
    />

    <template v-else>
      <div v-if="retention.policyKeys.value.length" class="retention-policies__selector">
        <AppSelectField
          :model-value="retention.selectedPolicyKey.value"
          label="Policy registrada"
          :options="policyOptions"
          searchable
          @update:model-value="retention.selectPolicy($event)"
        />
        <dl>
          <div>
            <dt>Publicada efetiva</dt>
            <dd>
              {{
                retention.latestPublished.value
                  ? `v${retention.latestPublished.value.version}`
                  : 'Nenhuma'
              }}
            </dd>
          </div>
          <div>
            <dt>Drafts</dt>
            <dd>{{ retention.selectedDrafts.value.length }}</dd>
          </div>
          <div>
            <dt>Versoes</dt>
            <dd>{{ retention.selectedVersions.value.length }}</dd>
          </div>
        </dl>
      </div>
      <CustomerIntelligenceStatus
        v-else
        title="Nenhuma policy registrada"
        empty
        empty-text="Crie um draft abaixo. Ele permanecera sem efeito ate uma publicacao explicita."
      />

      <div v-if="retention.selectedVersions.value.length" class="retention-policies__versions">
        <article
          v-for="version in retention.selectedVersions.value"
          :key="version.id"
          :class="`is-${version.status}`"
        >
          <header>
            <strong>v{{ version.version }}</strong>
            <span>{{ version.status === 'published' ? 'Publicada' : 'Draft' }}</span>
          </header>
          <dl>
            <div>
              <dt>Prazo</dt>
              <dd>{{ formatDays(version.snapshotTtlSeconds) }}</dd>
            </div>
            <div>
              <dt>Expiracao</dt>
              <dd>{{ expiryLabel(version.onExpiry) }}</dd>
            </div>
            <div>
              <dt>Revisao</dt>
              <dd>{{ version.revision }}</dd>
            </div>
            <div>
              <dt>{{ version.status === 'published' ? 'Publicada em' : 'Criada em' }}</dt>
              <dd>{{ formatDate(version.publishedAt || version.createdAt) }}</dd>
            </div>
          </dl>
          <p v-if="version.status === 'published'">
            {{ version.publicationReasonCode || 'motivo nao informado' }} ·
            {{ version.approvalReference || 'referencia nao informada' }}
          </p>
        </article>
      </div>

      <div
        v-if="retention.error.value && retention.policies.value.length"
        class="retention-policies__inline-error"
        role="alert"
      >
        {{ retention.error.value.message }}
      </div>

      <div v-if="retention.access.canManageSources.value" class="retention-policies__governance">
        <details>
          <summary>
            <span>
              <strong>Criar nova versao draft</strong>
              <small>Sem efeito operacional ate a publicacao separada.</small>
            </span>
          </summary>
          <form class="retention-policies__form" @submit.prevent="createDraft">
            <label>
              <span>Chave da policy</span>
              <input
                v-model.trim="draftPolicyKey"
                type="text"
                maxlength="160"
                pattern="[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*"
                autocomplete="off"
                required
              />
              <small>Minusculas, numeros e separadores ponto, hifen ou sublinhado.</small>
            </label>
            <label>
              <span>Prazo do snapshot (dias)</span>
              <input
                v-model.number="draftDays"
                type="number"
                min="1"
                max="3650"
                step="1"
                required
              />
            </label>
            <AppSelectField
              v-model="draftExpiry"
              label="Acao ao expirar"
              :options="[...RETENTION_EXPIRY_OPTIONS]"
            />
            <button type="submit" :disabled="!canCreateDraft">
              {{
                retention.savingAction.value === 'create_draft' ? 'Criando draft...' : 'Criar draft'
              }}
            </button>
          </form>
        </details>

        <details>
          <summary>
            <span>
              <strong>Publicar uma versao draft</strong>
              <small>Exige motivo fechado, aprovacao rastreavel e confirmacao.</small>
            </span>
          </summary>
          <form
            v-if="retention.selectedDrafts.value.length"
            class="retention-policies__form"
            @submit.prevent="publishDraft"
          >
            <AppSelectField
              :model-value="retention.selectedDraftId.value"
              label="Versao draft"
              :options="draftOptions"
              @update:model-value="retention.selectDraft($event)"
            />
            <AppSelectField
              v-model="publicationReason"
              label="Motivo da publicacao"
              :options="[...retention.reasonOptions]"
            />
            <label>
              <span>Referencia da aprovacao</span>
              <input
                v-model.trim="approvalReference"
                type="text"
                maxlength="128"
                pattern="[A-Za-z0-9][A-Za-z0-9._:-]{0,127}"
                placeholder="LEGAL-RETENTION-2026-001"
                autocomplete="off"
                required
              />
              <small>Use o ticket, parecer ou registro externo que autoriza a publicacao.</small>
            </label>
            <button type="submit" class="is-primary" :disabled="!canPublish">
              {{
                retention.savingAction.value === 'publish'
                  ? 'Publicando...'
                  : `Publicar v${retention.selectedDraft.value?.version ?? ''}`
              }}
            </button>
          </form>
          <p v-else class="retention-policies__empty-draft">
            A policy selecionada nao possui versao draft para publicar.
          </p>
        </details>
      </div>
    </template>
  </section>
</template>

<style scoped src="./intelligence-retention-policies.css"></style>
