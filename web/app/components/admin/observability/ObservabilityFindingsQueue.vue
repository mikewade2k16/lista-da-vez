<script setup lang="ts">
import { getApiErrorMessage } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'
import { useObservabilityApi } from '~/composables/useObservability'
import type { ObsFindingPatch, ObsFindingStatus, ObsSecurityFinding } from '~/types/observability'
import { SEVERITY_LABEL, severityBand } from '~/utils/observability/health'
import {
  countFindings,
  FINDING_FILTERS,
  FINDING_STATUS_OPTIONS,
  filterFindings,
  isFindingOverdue,
  NOTE_MAX,
  OWNER_MAX,
  todayISO,
  type FindingFilter,
} from '~/utils/observability/findings'

// Fila de trabalho dos achados da auditoria (task 2.4): status, responsavel, prazo e
// nota editaveis por linha. Salva com PATCH /findings/{id}; o servidor devolve o achado
// atualizado e o pai troca so aquela linha (sem recarregar a aba inteira).
const props = defineProps<{ findings: ObsSecurityFinding[] }>()
const emit = defineEmits<{ saved: [finding: ObsSecurityFinding] }>()

interface Draft {
  status: ObsFindingStatus
  owner: string
  dueDate: string
  note: string
}

const auth = useAuthStore()
const api = useObservabilityApi()

const me = computed(() => {
  const user = auth.user as { displayName?: string } | null
  const principal = auth.principal as { displayName?: string } | null
  return String(user?.displayName || principal?.displayName || '').trim()
})

const filter = ref<FindingFilter>('open')
const today = computed(() => todayISO())
const counters = computed(() => countFindings(props.findings, today.value))
const visible = computed(() => filterFindings(props.findings, filter.value, me.value, today.value))

const drafts = reactive<Record<string, Draft>>({})
const busyId = ref('')
const savedId = ref('')
const errors = reactive<Record<string, string>>({})
const noteOpen = reactive<Record<string, boolean>>({})

function fromFinding(finding: ObsSecurityFinding): Draft {
  return {
    status: finding.status as ObsFindingStatus,
    owner: finding.owner || '',
    dueDate: finding.dueDate || '',
    note: finding.note || '',
  }
}

// Rascunho da linha; recria quando o achado muda no servidor e a linha nao esta suja.
function draftOf(finding: ObsSecurityFinding): Draft {
  if (!drafts[finding.id]) drafts[finding.id] = fromFinding(finding)
  return drafts[finding.id]!
}

function buildPatch(finding: ObsSecurityFinding): ObsFindingPatch {
  const draft = draftOf(finding)
  const patch: ObsFindingPatch = {}
  if (draft.status !== finding.status) patch.status = draft.status
  if (draft.owner.trim() !== (finding.owner || '')) patch.owner = draft.owner.trim()
  if (draft.dueDate !== (finding.dueDate || '')) patch.dueDate = draft.dueDate || null
  if (draft.note.trim() !== (finding.note || '')) patch.note = draft.note.trim()
  return patch
}

function isDirty(finding: ObsSecurityFinding): boolean {
  return Object.keys(buildPatch(finding)).length > 0
}

function overdue(finding: ObsSecurityFinding): boolean {
  // Considera o prazo ainda nao salvo para o destaque acompanhar a edicao.
  const draft = draftOf(finding)
  return isFindingOverdue({ status: draft.status, dueDate: draft.dueDate || null }, today.value)
}

async function save(finding: ObsSecurityFinding) {
  const patch = buildPatch(finding)
  if (Object.keys(patch).length === 0) return
  busyId.value = finding.id
  savedId.value = ''
  errors[finding.id] = ''
  try {
    const updated = await api.patch<ObsSecurityFinding>(
      `/findings/${encodeURIComponent(finding.id)}`,
      patch as Record<string, unknown>,
    )
    drafts[finding.id] = fromFinding(updated)
    emit('saved', updated)
    savedId.value = finding.id
    setTimeout(() => {
      if (savedId.value === finding.id) savedId.value = ''
    }, 2500)
  } catch (cause) {
    errors[finding.id] = getApiErrorMessage(cause, 'Não foi possível salvar o achado.')
  } finally {
    busyId.value = ''
  }
}

function emptyMessage(): string {
  if (filter.value === 'mine' && !me.value) return 'Não foi possível identificar o seu nome.'
  return 'Nenhum achado neste filtro.'
}
</script>

<template>
  <section class="obs-section">
    <h3 class="obs-section__title">Achados da auditoria</h3>

    <div class="obs-cards">
      <div
        v-for="severity in ['critical', 'high', 'medium', 'low'] as const"
        :key="severity"
        class="obs-card"
      >
        <span class="obs-card__label">Abertos: {{ SEVERITY_LABEL[severity] }}</span>
        <span class="obs-card__value">{{ counters.open[severity] }}</span>
      </div>
      <div class="obs-card">
        <span class="obs-card__label">Vencidos</span>
        <span :class="['obs-card__value', counters.overdue > 0 ? 'obs-text--bad' : '']">
          {{ counters.overdue }}
        </span>
      </div>
    </div>

    <div class="obs-toolbar">
      <AppSegmentedFilter
        v-model="filter"
        :options="FINDING_FILTERS.map((f) => ({ value: f.key, label: f.label }))"
        aria-label="Filtro dos achados"
      />
    </div>

    <p v-if="visible.length === 0" class="obs-hint">{{ emptyMessage() }}</p>
    <div v-else class="obs-tablewrap">
      <table class="obs-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Severidade</th>
            <th>Achado</th>
            <th>Status</th>
            <th>Responsável</th>
            <th>Prazo</th>
            <th>Ações</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="finding in visible" :key="finding.id">
            <tr :class="{ 'obs-row--regression': overdue(finding) }">
              <td class="obs-mono">{{ finding.id }}</td>
              <td>
                <span :class="['obs-band', `obs-band--${severityBand(finding.severity)}`]">
                  {{ SEVERITY_LABEL[finding.severity] || finding.severity }}
                </span>
              </td>
              <td>
                <div>{{ finding.title }}</div>
                <div class="obs-muted obs-mono">{{ finding.location || finding.area }}</div>
                <div v-if="finding.updatedBy" class="obs-muted">
                  Alterado por {{ finding.updatedBy }}
                </div>
              </td>
              <td>
                <USelect
                  v-model="draftOf(finding).status"
                  :items="FINDING_STATUS_OPTIONS"
                  size="xs"
                  :aria-label="`Status do achado ${finding.id}`"
                />
              </td>
              <td>
                <UInput
                  v-model="draftOf(finding).owner"
                  size="xs"
                  placeholder="Responsável"
                  :maxlength="OWNER_MAX"
                  :aria-label="`Responsável pelo achado ${finding.id}`"
                />
              </td>
              <td>
                <UInput
                  v-model="draftOf(finding).dueDate"
                  type="date"
                  size="xs"
                  :class="{ 'obs-text--bad': overdue(finding) }"
                  :aria-label="`Prazo do achado ${finding.id}`"
                />
                <span v-if="overdue(finding)" class="obs-text--bad">Vencido</span>
              </td>
              <td>
                <div class="obs-toolbar">
                  <UButton
                    label="Nota"
                    icon="i-lucide-notebook-pen"
                    size="xs"
                    color="neutral"
                    :variant="draftOf(finding).note ? 'soft' : 'ghost'"
                    @click="noteOpen[finding.id] = !noteOpen[finding.id]"
                  />
                  <UButton
                    label="Salvar"
                    size="xs"
                    :disabled="!isDirty(finding)"
                    :loading="busyId === finding.id"
                    @click="save(finding)"
                  />
                  <span v-if="savedId === finding.id" class="obs-text--good">Salvo</span>
                </div>
                <div v-if="errors[finding.id]" class="obs-text--bad">{{ errors[finding.id] }}</div>
              </td>
            </tr>
            <tr v-if="noteOpen[finding.id]">
              <td colspan="7">
                <UTextarea
                  v-model="draftOf(finding).note"
                  :rows="3"
                  :maxlength="NOTE_MAX"
                  placeholder="Nota sobre o achado (decisão, link, contexto)"
                  class="w-full"
                  :aria-label="`Nota do achado ${finding.id}`"
                />
                <span class="obs-muted">{{ draftOf(finding).note.length }}/{{ NOTE_MAX }}</span>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </section>
</template>
