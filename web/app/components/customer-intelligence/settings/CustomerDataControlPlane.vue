<script setup lang="ts">
import { reactive } from 'vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type { useCustomerControlPlane } from '~/composables/customer-intelligence/useCustomerControlPlane'
import {
  CUSTOMER_DATA_CAPABILITY_DEFINITIONS,
  CUSTOMER_DATA_WRITER_DEFINITIONS,
  type CustomerDataCapabilityKey,
  type CustomerDataWriterKey,
  type CustomerDataWriterMode,
} from '~/domain/customer-intelligence/control-plane-types'

const props = defineProps<{
  control: ReturnType<typeof useCustomerControlPlane>
}>()
const control = props.control

const capabilityReasons = reactive<Record<CustomerDataCapabilityKey, string>>({
  core: '',
  identity_resolution: '',
  matching_merge: '',
  offline_interactions: '',
  segmentation: '',
  segment_exports: '',
})
const writerReasons = reactive<Record<CustomerDataWriterKey, string>>({
  relationship: '',
  identity: '',
  note: '',
  consent: '',
  merge: '',
  segment_definition: '',
})
const writerWatermarks = reactive<Record<CustomerDataWriterKey, string>>({
  relationship: '',
  identity: '',
  note: '',
  consent: '',
  merge: '',
  segment_definition: '',
})
const writerSourceChecksums = reactive<Record<CustomerDataWriterKey, string>>({
  relationship: '',
  identity: '',
  note: '',
  consent: '',
  merge: '',
  segment_definition: '',
})
const writerTargetChecksums = reactive<Record<CustomerDataWriterKey, string>>({
  relationship: '',
  identity: '',
  note: '',
  consent: '',
  merge: '',
  segment_definition: '',
})
const writerConfirmations = reactive<Record<CustomerDataWriterKey, boolean>>({
  relationship: false,
  identity: false,
  note: false,
  consent: false,
  merge: false,
  segment_definition: false,
})

function selectedWriterMode(key: CustomerDataWriterKey): CustomerDataWriterMode {
  return (
    control.customerDataWriterDrafts.value[key] ?? control.customerDataWriter(key)?.mode ?? 'legacy'
  )
}

function capabilitySaveDisabled(key: CustomerDataCapabilityKey): boolean {
  const current = control.customerDataCapability(key)
  const draft = control.customerDataCapabilityDrafts.value[key]
  return (
    !current ||
    !draft ||
    draft === current.mode ||
    !capabilityReasons[key].trim() ||
    control.savingCustomerDataCapabilityKey.value === key
  )
}

function writerSaveDisabled(key: CustomerDataWriterKey): boolean {
  const current = control.customerDataWriter(key)
  const draft = selectedWriterMode(key)
  if (
    !current ||
    draft === current.mode ||
    !writerReasons[key].trim() ||
    control.savingCustomerDataWriterKey.value === key
  ) {
    return true
  }
  if (draft !== 'new') return false
  const sourceChecksum = writerSourceChecksums[key].trim()
  const targetChecksum = writerTargetChecksums[key].trim()
  return !writerConfirmations[key] || !sourceChecksum || sourceChecksum !== targetChecksum
}

async function saveCapability(key: CustomerDataCapabilityKey): Promise<void> {
  const saved = await control.saveCustomerDataCapability(key, capabilityReasons[key])
  if (saved) capabilityReasons[key] = ''
}

async function saveWriter(key: CustomerDataWriterKey): Promise<void> {
  const saved = await control.saveCustomerDataWriter(key, {
    reason: writerReasons[key],
    watermark: writerWatermarks[key],
    sourceChecksum: writerSourceChecksums[key],
    targetChecksum: writerTargetChecksums[key],
  })
  if (!saved) return
  writerReasons[key] = ''
  writerConfirmations[key] = false
}
</script>

<template>
  <section v-if="control.access.hasCustomerDataModule.value" class="control-section">
    <header>
      <div>
        <small>Customer Data - deterministico</small>
        <h2>Capabilities e writers</h2>
        <p>
          Ativacao separada por cliente e entidade, com revisao otimista, idempotencia, motivo
          auditavel e cutover progressivo.
        </p>
      </div>
      <span class="control-badge">API administrativa real</span>
    </header>

    <CustomerIntelligenceStatus
      v-if="!control.access.canManageCustomerDataCapabilities.value"
      title="Permissao administrativa necessaria"
      :error="{
        kind: 'forbidden',
        message: '',
        reasonCode: 'customer_data_capabilities_manage_required',
        statusCode: 403,
      }"
    />
    <CustomerIntelligenceStatus
      v-else-if="!control.access.clientScopeReady.value"
      title="Selecione o cliente"
      empty
      empty-text="Escolha um cliente autorizado antes de consultar ou alterar estados."
    />
    <template v-else>
      <CustomerIntelligenceStatus
        v-if="control.customerDataLoading.value"
        title="Carregando estados do Customer Data"
        loading
      />
      <CustomerIntelligenceStatus
        v-if="control.customerDataError.value"
        title="Nao foi possivel concluir a operacao"
        :error="control.customerDataError.value"
        @retry="control.loadCustomerData"
      />

      <div
        v-if="control.customerDataState.value && !control.customerDataLoading.value"
        class="control-groups"
      >
        <div class="control-group">
          <div>
            <h3>Capabilities</h3>
            <p>
              Capabilities nao-core devem passar por shadow antes de on. Cada alteracao exige
              justificativa.
            </p>
          </div>
          <div class="control-cards">
            <article
              v-for="definition in CUSTOMER_DATA_CAPABILITY_DEFINITIONS"
              :key="definition.key"
            >
              <div>
                <small>{{ definition.key }}</small>
                <h3>{{ definition.label }}</h3>
                <p>{{ definition.description }}</p>
                <span class="control-revision">
                  Revisao {{ control.customerDataCapability(definition.key)?.revision ?? 0 }}
                </span>
              </div>
              <AppSelectField
                :model-value="
                  control.customerDataCapabilityDrafts.value[definition.key] ??
                  control.customerDataCapability(definition.key)?.mode ??
                  'off'
                "
                :options="
                  control.customerDataCapabilityModes(definition.key).map((mode) => ({
                    value: mode,
                    label: mode,
                  }))
                "
                label="Modo"
                @update:model-value="control.setCustomerDataCapabilityDraft(definition.key, $event)"
              />
              <label class="control-field">
                <span>Motivo da alteracao</span>
                <input
                  v-model="capabilityReasons[definition.key]"
                  type="text"
                  maxlength="1000"
                  placeholder="Ex.: validacao concluida no ambiente shadow"
                />
              </label>
              <button
                type="button"
                :disabled="capabilitySaveDisabled(definition.key)"
                @click="saveCapability(definition.key)"
              >
                {{
                  control.savingCustomerDataCapabilityKey.value === definition.key
                    ? 'Salvando...'
                    : 'Aplicar estado'
                }}
              </button>
            </article>
          </div>
        </div>

        <div class="control-group">
          <div>
            <h3>Writer unico por entidade</h3>
            <p>
              Fluxo permitido: legacy para shadow, shadow para legacy ou new e new para shadow. O
              corte para new exige capability on e checksums iguais.
            </p>
          </div>
          <div class="control-cards">
            <article v-for="definition in CUSTOMER_DATA_WRITER_DEFINITIONS" :key="definition.key">
              <div>
                <small>{{ definition.key }} - capability {{ definition.capabilityKey }}</small>
                <h3>{{ definition.label }}</h3>
                <span class="control-revision">
                  Revisao {{ control.customerDataWriter(definition.key)?.revision ?? 0 }}
                </span>
              </div>
              <AppSelectField
                :model-value="selectedWriterMode(definition.key)"
                :options="
                  control.customerDataWriterModes(definition.key).map((mode) => ({
                    value: mode,
                    label: mode,
                  }))
                "
                label="Writer"
                @update:model-value="control.setCustomerDataWriterDraft(definition.key, $event)"
              />
              <label class="control-field">
                <span>Watermark (opcional)</span>
                <input
                  v-model="writerWatermarks[definition.key]"
                  type="text"
                  :placeholder="
                    control.customerDataWriter(definition.key)?.watermark ||
                    'Marcador da reconciliacao'
                  "
                />
              </label>
              <template v-if="selectedWriterMode(definition.key) === 'new'">
                <label class="control-field">
                  <span>Checksum da origem</span>
                  <input
                    v-model="writerSourceChecksums[definition.key]"
                    type="text"
                    autocomplete="off"
                    placeholder="Hash obtido na reconciliacao"
                  />
                </label>
                <label class="control-field">
                  <span>Checksum do destino</span>
                  <input
                    v-model="writerTargetChecksums[definition.key]"
                    type="text"
                    autocomplete="off"
                    placeholder="Deve ser identico ao hash de origem"
                  />
                </label>
                <label class="control-confirmation">
                  <input v-model="writerConfirmations[definition.key]" type="checkbox" />
                  <span>Confirmo a reconciliacao e o cutover para o novo writer.</span>
                </label>
              </template>
              <label class="control-field">
                <span>Motivo da alteracao</span>
                <input
                  v-model="writerReasons[definition.key]"
                  type="text"
                  maxlength="1000"
                  placeholder="Justificativa registrada na auditoria"
                />
              </label>
              <button
                type="button"
                :disabled="writerSaveDisabled(definition.key)"
                @click="saveWriter(definition.key)"
              >
                {{
                  control.savingCustomerDataWriterKey.value === definition.key
                    ? 'Salvando...'
                    : 'Aplicar writer'
                }}
              </button>
            </article>
          </div>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.control-section,
.control-groups,
.control-group {
  display: grid;
  gap: 1rem;
}

.control-section {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
}

.control-section > header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.control-section h2,
.control-section h3,
.control-section p {
  margin: 0;
}

.control-section p,
.control-section small,
.control-revision {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.control-badge {
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
  font-size: 0.68rem;
  font-weight: 700;
}

.control-cards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.control-cards article {
  display: grid;
  align-content: start;
  gap: 0.65rem;
  padding: 0.8rem;
  border: 1px solid rgb(var(--border) / 0.7);
  border-radius: 0.75rem;
}

.control-field,
.control-confirmation {
  display: grid;
  gap: 0.3rem;
  color: rgb(var(--muted));
  font-size: 0.7rem;
  font-weight: 700;
}

.control-field input {
  width: 100%;
  min-height: 2.45rem;
  padding: 0.55rem 0.65rem;
  border: 1px solid rgb(var(--border) / 0.9);
  border-radius: 0.6rem;
  background: rgb(var(--surface-1));
  color: rgb(var(--text));
}

.control-confirmation {
  grid-template-columns: auto 1fr;
  align-items: start;
  font-weight: 500;
}

.control-confirmation input {
  margin-top: 0.15rem;
}

@media (max-width: 760px) {
  .control-cards {
    grid-template-columns: 1fr;
  }
}
</style>
