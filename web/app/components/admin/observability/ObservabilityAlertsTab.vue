<script setup lang="ts">
import AppSegmentedFilter from '~/components/ui/AppSegmentedFilter.vue'
import ObservabilitySettingsPanel from '~/components/admin/observability/ObservabilitySettingsPanel.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery, useObservabilityApi } from '~/composables/useObservability'
import type { ObsAlert } from '~/types/observability'
import { getApiErrorMessage } from '~/utils/api-client'
import {
  alertControlBadge,
  alertControlDetail,
  isSilenced,
  SILENCE_OPTIONS,
} from '~/utils/observability/alerts'
import { formatAge, SEVERITY_LABEL, severityBand } from '~/utils/observability/health'
import { formatDate } from '~/utils/observability/format'

const VIEWS = [
  { value: 'alerts', label: 'Alertas' },
  { value: 'settings', label: 'Configurações' },
]
const view = ref('alerts')

const STATUSES = [
  { value: 'open', label: 'Abertos' },
  { value: 'resolved', label: 'Resolvidos' },
  { value: 'all', label: 'Todos' },
]
const status = ref('open')
const { data, loading, error, reload } = useObsQuery<ObsAlert[]>('/alerts', () => ({
  status: status.value,
}))

const api = useObservabilityApi()
const sending = ref(false)
const testMessage = ref('')
const testFailed = ref(false)

function isNotConfigured(cause: unknown): boolean {
  const err = cause as {
    statusCode?: number
    status?: number
    data?: { error?: { code?: string } }
  }
  return (
    err?.data?.error?.code === 'alerts_not_configured' || (err?.statusCode ?? err?.status) === 409
  )
}

// Reconhecer / silenciar / reativar avisos (so alertas abertos). Recarrega a lista
// depois de cada acao; o erro aparece na barra de ferramentas.
const busyId = ref<number | null>(null)
const actionError = ref('')

async function control(alert: ObsAlert, action: 'ack' | 'silence' | 'unsilence', hours?: number) {
  busyId.value = alert.id
  actionError.value = ''
  try {
    await api.post(`/alerts/${alert.id}/${action}`, action === 'silence' ? { hours } : {})
    await reload()
  } catch (cause) {
    actionError.value = getApiErrorMessage(cause, 'Não foi possível atualizar o alerta.')
  } finally {
    busyId.value = null
  }
}

function silenceItems(alert: ObsAlert) {
  return SILENCE_OPTIONS.map((option) => ({
    label: option.label,
    onSelect: () => void control(alert, 'silence', option.hours),
  }))
}

async function sendTest() {
  sending.value = true
  testMessage.value = ''
  try {
    await api.post('/alerts/test', {})
    testFailed.value = false
    testMessage.value = 'E-mail de teste enviado aos destinatários configurados.'
  } catch (cause) {
    testFailed.value = true
    testMessage.value = isNotConfigured(cause)
      ? 'Avisos por e-mail não configurados: defina o SMTP e OBS_ALERT_EMAILS no ambiente da API.'
      : getApiErrorMessage(cause, 'Não foi possível enviar o e-mail de teste.')
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div class="obs-stack">
    <AppSegmentedFilter v-model="view" :options="VIEWS" aria-label="Seção de alertas" />

    <ObservabilitySettingsPanel v-if="view === 'settings'" />

    <div v-if="view === 'alerts'" class="obs-toolbar">
      <AppSegmentedFilter v-model="status" :options="STATUSES" aria-label="Status do alerta" />
      <UButton
        label="Enviar e-mail de teste"
        icon="i-lucide-mail"
        size="sm"
        color="neutral"
        variant="soft"
        :loading="sending"
        @click="sendTest"
      />
      <span v-if="actionError" class="obs-muted obs-text--bad">{{ actionError }}</span>
      <span
        v-if="testMessage"
        :class="['obs-muted', testFailed ? 'obs-text--bad' : 'obs-text--good']"
      >
        {{ testMessage }}
      </span>
    </div>

    <ObservabilityState
      v-if="view === 'alerts'"
      :loading="loading"
      :error="error"
      :empty="!data || data.length === 0"
      empty-title="Nenhum alerta neste filtro."
      empty-hint="Alertas abrem sozinhos quando uma regra de saúde é violada."
      @retry="reload"
    >
      <div class="obs-tablewrap">
        <table class="obs-table">
          <thead>
            <tr>
              <th>Severidade</th>
              <th>Alerta</th>
              <th>Área</th>
              <th>Status</th>
              <th>Aberto</th>
              <th>Último sinal</th>
              <th>Resolvido</th>
              <th class="obs-num">Avisos</th>
              <th>Ações</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="alert in data || []" :key="alert.id">
              <td>
                <span :class="['obs-band', `obs-band--${severityBand(alert.severity)}`]">
                  {{ SEVERITY_LABEL[alert.severity] || alert.severity }}
                </span>
              </td>
              <td>
                <strong>{{ alert.title }}</strong>
                <div class="obs-muted">{{ alert.detail?.measured }}</div>
                <div v-if="alertControlDetail(alert)" class="obs-muted">
                  {{ alertControlDetail(alert) }}
                </div>
                <div v-if="alert.lastNotifyError" class="obs-text--bad obs-muted">
                  Falha no aviso: {{ alert.lastNotifyError }}
                </div>
              </td>
              <td>{{ alert.area }}</td>
              <td>
                {{ alert.status === 'open' ? 'Aberto' : 'Resolvido' }}
                <span
                  v-if="alertControlBadge(alert)"
                  :class="[
                    'obs-band',
                    alertControlBadge(alert)?.kind === 'silenced'
                      ? 'obs-band--warn'
                      : 'obs-band--good',
                  ]"
                >
                  {{ alertControlBadge(alert)?.text }}
                </span>
              </td>
              <td :title="formatDate(alert.firstSeen)">{{ formatAge(alert.firstSeen) }}</td>
              <td>{{ formatAge(alert.lastSeen) }}</td>
              <td>{{ formatDate(alert.resolvedAt) }}</td>
              <td class="obs-num">{{ alert.notifyCount }}</td>
              <td>
                <div v-if="alert.status === 'open'" class="obs-toolbar">
                  <UButton
                    v-if="!alert.acknowledgedAt"
                    label="Reconhecer"
                    size="xs"
                    color="neutral"
                    variant="soft"
                    :loading="busyId === alert.id"
                    @click="control(alert, 'ack')"
                  />
                  <UButton
                    v-if="isSilenced(alert)"
                    label="Reativar avisos"
                    size="xs"
                    color="neutral"
                    variant="soft"
                    :loading="busyId === alert.id"
                    @click="control(alert, 'unsilence')"
                  />
                  <UDropdownMenu v-else :items="silenceItems(alert)" :content="{ align: 'end' }">
                    <UButton
                      label="Silenciar"
                      icon="i-lucide-bell-off"
                      size="xs"
                      color="neutral"
                      variant="soft"
                      :loading="busyId === alert.id"
                    />
                  </UDropdownMenu>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </ObservabilityState>
  </div>
</template>
