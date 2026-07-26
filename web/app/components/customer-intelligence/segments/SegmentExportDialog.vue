<script setup lang="ts">
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type {
  SegmentExportDescriptor,
  SegmentExportView,
  SegmentMaterializationView,
} from '~/domain/customer-data/segment-types'

const props = defineProps<{
  open: boolean
  descriptor?: SegmentExportDescriptor
  materialization: SegmentMaterializationView | null
  exportMode: 'off' | 'shadow' | 'canary' | 'on'
  busy: boolean
  result: SegmentExportView | null
}>()

const emit = defineEmits<{
  close: []
  submit: [
    input: {
      materializationId: string
      purposeKey: string
      channelKey: string
      formatKey: string
      fieldSetKey: string
      reason?: string
    },
  ]
}>()

const purposeKey = ref('')
const channelKey = ref('')
const formatKey = ref('')
const fieldSetKey = ref('')
const reason = ref('')

watch(
  () => props.open,
  (open) => {
    if (!open) return
    purposeKey.value = ''
    channelKey.value = ''
    formatKey.value = ''
    fieldSetKey.value = ''
    reason.value = ''
  },
)

const complete = computed(
  () =>
    Boolean(props.materialization) &&
    Boolean(purposeKey.value) &&
    Boolean(channelKey.value) &&
    Boolean(formatKey.value) &&
    Boolean(fieldSetKey.value) &&
    (!props.descriptor?.requiresReason || Boolean(reason.value.trim())),
)

function submit(): void {
  if (!complete.value || !props.materialization) return
  emit('submit', {
    materializationId: props.materialization.id,
    purposeKey: purposeKey.value,
    channelKey: channelKey.value,
    formatKey: formatKey.value,
    fieldSetKey: fieldSetKey.value,
    reason: reason.value.trim() || undefined,
  })
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="export-dialog" role="dialog" aria-modal="true">
      <section>
        <header>
          <div>
            <h2>Elegibilidade de exportacao</h2>
            <p>
              Pertencer ao segmento nao equivale a consentimento. Este fluxo nunca envia mensagem
              nem cria campanha.
            </p>
          </div>
          <button type="button" @click="emit('close')">Fechar</button>
        </header>

        <CustomerIntelligenceStatus
          v-if="exportMode === 'off'"
          title="Exportacao desabilitada"
          :error="{
            kind: 'capability_off',
            message: '',
            reasonCode: 'segment_exports_off',
            statusCode: 0,
          }"
        />
        <CustomerIntelligenceStatus
          v-else-if="!descriptor"
          title="Catalogo de exportacao indisponivel"
          empty
          empty-text="O backend precisa fornecer finalidade, canal, formato e field set allowlisted."
        />
        <div v-else class="export-dialog__form">
          <AppSelectField
            v-model="purposeKey"
            label="Finalidade"
            :options="descriptor.purposeOptions"
          />
          <AppSelectField v-model="channelKey" label="Canal" :options="descriptor.channelOptions" />
          <AppSelectField v-model="formatKey" label="Formato" :options="descriptor.formatOptions" />
          <AppSelectField
            v-model="fieldSetKey"
            label="Conjunto de campos"
            :options="descriptor.fieldSetOptions"
          />
          <label v-if="descriptor.requiresReason">
            Motivo
            <input v-model="reason" type="text" maxlength="240" />
          </label>
          <button type="button" :disabled="!complete || busy" @click="submit">
            {{
              exportMode === 'shadow' ? 'Gerar relatorio de elegibilidade' : 'Solicitar exportacao'
            }}
          </button>
        </div>

        <div v-if="result" class="export-dialog__result">
          <strong>{{ result.status }}</strong>
          <span>Candidatos: {{ result.candidateCount ?? 'protegido' }}</span>
          <span>Elegiveis: {{ result.eligibleCount ?? 'protegido' }}</span>
          <span>Excluidos: {{ result.excludedCount ?? 'protegido' }}</span>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.export-dialog {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 1rem;
  background: rgb(0 0 0 / 0.5);
}

.export-dialog > section {
  display: grid;
  gap: 1rem;
  width: min(46rem, 100%);
  max-height: 90vh;
  overflow: auto;
  padding: 1.2rem;
  border-radius: 1rem;
  background: rgb(var(--surface));
}

.export-dialog header,
.export-dialog__result {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.export-dialog h2,
.export-dialog p {
  margin: 0;
}

.export-dialog p,
.export-dialog__result {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}

.export-dialog__form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.export-dialog__form label {
  display: grid;
  gap: 0.3rem;
}

@media (max-width: 720px) {
  .export-dialog__form {
    grid-template-columns: 1fr;
  }
}
</style>
