<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { fetchQueues } from '~/domain/omnichannel/config-api'
import type { OmniInstance, OmniQueue } from '~/domain/omnichannel/config-types'
import {
  fetchOperationalHealth,
  fetchRolloutConfig,
  putRolloutConfig,
  type OperationalHealth,
  type RolloutConfig,
  type RolloutMode,
} from '~/domain/omnichannel/operations-api'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const props = defineProps<{ instances: OmniInstance[]; canManage: boolean }>()
const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

type WindowDraft = { days: string; start: string; end: string }
type Draft = {
  mode: RolloutMode
  allowedInstanceIds: string[]
  allowedQueueIds: string[]
  autoReplyPercent: number
  timezone: string
  windows: WindowDraft[]
  excludedTags: string
  maxDailyAutoReplies: number
  killSwitchReason: string
  reason: string
}

const health = ref<OperationalHealth | null>(null)
const config = ref<RolloutConfig | null>(null)
const queues = ref<OmniQueue[]>([])
const loading = ref(true)
const saving = ref(false)
const draft = reactive<Draft>({
  mode: 'active',
  allowedInstanceIds: [],
  allowedQueueIds: [],
  autoReplyPercent: 100,
  timezone: 'America/Sao_Paulo',
  windows: [],
  excludedTags: '',
  maxDailyAutoReplies: 0,
  killSwitchReason: '',
  reason: '',
})

const modeHelp: Record<RolloutMode, string> = {
  off: 'Canal sem IA.',
  observe: 'Recebe e mede, sem executar IA.',
  shadow: 'Executa IA para avaliação, sem efeito operacional.',
  assist: 'Gera decisão/draft, sem envio automático.',
  auto_pilot: 'Envia somente para o coorte e as regras abaixo.',
  active: 'Operação completa; preserva o comportamento atual.',
  paused: 'Kill switch: bloqueia IA e envio, mantendo o inbox humano.',
}

const statusLabel = computed(() => (health.value?.status === 'ok' ? 'Saudável' : 'Atenção'))

function applyConfig(value: RolloutConfig): void {
  config.value = value
  draft.mode = value.mode
  draft.allowedInstanceIds = [...value.allowedInstanceIds]
  draft.allowedQueueIds = [...value.allowedQueueIds]
  draft.autoReplyPercent = value.autoReplyPercent
  draft.timezone = value.allowedHours.timezone
  draft.windows = value.allowedHours.windows.map((window) => ({
    days: window.days.join(','),
    start: window.start,
    end: window.end,
  }))
  draft.excludedTags = value.excludedTags.join(', ')
  draft.maxDailyAutoReplies = value.maxDailyAutoReplies
  draft.killSwitchReason = value.killSwitchReason || ''
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const [loadedHealth, loadedConfig, loadedQueues] = await Promise.all([
      fetchOperationalHealth(api),
      fetchRolloutConfig(api),
      props.canManage ? fetchQueues(api) : Promise.resolve([]),
    ])
    health.value = loadedHealth
    queues.value = loadedQueues.filter((queue) => queue.isActive)
    applyConfig(loadedConfig)
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar saúde e rollout.'))
  } finally {
    loading.value = false
  }
}

function toggle(values: string[], id: string): void {
  const index = values.indexOf(id)
  if (index >= 0) values.splice(index, 1)
  else values.push(id)
}

function addWindow(): void {
  draft.windows.push({ days: '1,2,3,4,5', start: '09:00', end: '18:00' })
}

async function save(): Promise<void> {
  if (!props.canManage || saving.value || !config.value || draft.reason.trim().length < 3) return
  if (draft.mode === 'paused' && draft.killSwitchReason.trim().length < 3) return
  saving.value = true
  try {
    const updated = await putRolloutConfig(api, {
      mode: draft.mode,
      allowedInstanceIds: [...draft.allowedInstanceIds],
      allowedInstagramAccountIds: [...config.value.allowedInstagramAccountIds],
      allowedQueueIds: [...draft.allowedQueueIds],
      autoReplyPercent: Number(draft.autoReplyPercent),
      allowedHours: {
        timezone: draft.timezone.trim(),
        windows: draft.windows.map((window) => ({
          days: window.days
            .split(',')
            .map((day) => Number(day.trim()))
            .filter((day) => Number.isInteger(day)),
          start: window.start,
          end: window.end,
        })),
      },
      excludedTags: draft.excludedTags
        .split(',')
        .map((tag) => tag.trim())
        .filter(Boolean),
      maxDailyAutoReplies: Number(draft.maxDailyAutoReplies),
      killSwitchReason: draft.mode === 'paused' ? draft.killSwitchReason.trim() : null,
      expectedRevision: config.value.revision,
      reason: draft.reason.trim(),
    })
    applyConfig(updated)
    draft.reason = ''
    ui.success(updated.mode === 'paused' ? 'Kill switch aplicado.' : 'Rollout atualizado.')
    health.value = await fetchOperationalHealth(api)
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível atualizar o rollout.'))
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="ops-config">
    <header class="ops-config__header">
      <div>
        <h3>Saúde e rollout</h3>
        <p>Gates autoritativos do backend. O n8n não envia nem promove o modo.</p>
      </div>
      <AppPanelButton variant="secondary" :disabled="loading" @click="load">
        Atualizar
      </AppPanelButton>
    </header>

    <p v-if="loading" class="ops-config__muted">Carregando estado operacional…</p>
    <template v-else-if="health && config">
      <div class="ops-config__summary" :class="`is-${health.status}`">
        <strong>{{ statusLabel }}</strong>
        <span>Outbox {{ health.outbox.pending }} pendente(s) · {{ health.outbox.dead }} dead</span>
        <span>IA {{ health.ai.queued }} na fila · {{ health.ai.stuckProcessing }} presa(s)</span>
        <span>{{ health.provider.webhookEvents24h }} webhook(s) em 24h</span>
      </div>

      <div v-if="health.alerts.length" class="ops-config__alerts">
        <article v-for="alert in health.alerts" :key="alert.code" :class="`is-${alert.severity}`">
          <strong>{{ alert.message }}</strong>
          <span>{{ alert.action }}</span>
          <small>{{ alert.owner }} · {{ alert.code }}</small>
        </article>
      </div>

      <div class="ops-config__form">
        <label>
          <span>Modo</span>
          <select v-model="draft.mode" :disabled="!canManage || saving">
            <option value="off">Off</option>
            <option value="observe">Observe</option>
            <option value="shadow">Shadow</option>
            <option value="assist">Assist</option>
            <option value="auto_pilot">Auto piloto</option>
            <option value="active">Ativo</option>
            <option value="paused">Pausado (kill switch)</option>
          </select>
          <small>{{ modeHelp[draft.mode] }}</small>
        </label>

        <div v-if="config.legacyDefault" class="ops-config__legacy">
          Esta conta ainda usa o fallback legado ativo. Salve uma configuração explícita antes do
          piloto.
        </div>

        <label v-if="draft.mode === 'paused'">
          <span>Motivo do kill switch</span>
          <textarea v-model="draft.killSwitchReason" rows="2" maxlength="500"></textarea>
        </label>

        <div class="ops-config__grid">
          <label>
            <span>Percentual automático</span>
            <input v-model.number="draft.autoReplyPercent" type="number" min="0" max="100" />
          </label>
          <label>
            <span>Máximo diário (0 = sem teto)</span>
            <input v-model.number="draft.maxDailyAutoReplies" type="number" min="0" />
          </label>
        </div>

        <fieldset>
          <legend>Números permitidos (vazio = todos)</legend>
          <label v-for="instance in instances" :key="instance.id" class="ops-config__check">
            <input
              type="checkbox"
              :checked="draft.allowedInstanceIds.includes(instance.id)"
              :disabled="!canManage || saving"
              @change="toggle(draft.allowedInstanceIds, instance.id)"
            />
            <span>{{ instance.displayName || instance.instanceName }}</span>
          </label>
        </fieldset>

        <fieldset>
          <legend>Filas permitidas (vazio = todas)</legend>
          <label v-for="queue in queues" :key="queue.id" class="ops-config__check">
            <input
              type="checkbox"
              :checked="draft.allowedQueueIds.includes(queue.id)"
              :disabled="!canManage || saving"
              @change="toggle(draft.allowedQueueIds, queue.id)"
            />
            <span>{{ queue.name }}</span>
          </label>
        </fieldset>

        <label>
          <span>Tags excluídas (separadas por vírgula)</span>
          <input v-model="draft.excludedTags" type="text" placeholder="vip, legal, reclamação" />
        </label>

        <div class="ops-config__schedule">
          <label>
            <span>Fuso</span>
            <input v-model="draft.timezone" type="text" />
          </label>
          <article v-for="(window, index) in draft.windows" :key="index" class="ops-config__window">
            <input
              v-model="window.days"
              type="text"
              aria-label="Dias 0 a 6"
              placeholder="1,2,3,4,5"
            />
            <input v-model="window.start" type="time" aria-label="Início" />
            <input v-model="window.end" type="time" aria-label="Fim" />
            <button type="button" @click="draft.windows.splice(index, 1)">Remover</button>
          </article>
          <AppPanelButton variant="secondary" :disabled="!canManage" @click="addWindow">
            Adicionar janela
          </AppPanelButton>
          <small>Dias: 0 domingo, 1 segunda … 6 sábado. Sem janela = qualquer horário.</small>
        </div>

        <label>
          <span>Motivo desta alteração</span>
          <textarea
            v-model="draft.reason"
            rows="2"
            maxlength="500"
            placeholder="Obrigatório para auditoria"
          ></textarea>
        </label>

        <div class="ops-config__actions">
          <span>Revisão {{ config.revision }}</span>
          <AppPanelButton
            :disabled="
              !canManage ||
              saving ||
              draft.reason.trim().length < 3 ||
              (draft.mode === 'paused' && draft.killSwitchReason.trim().length < 3)
            "
            @click="save"
          >
            {{
              saving
                ? 'Salvando…'
                : draft.mode === 'paused'
                  ? 'Aplicar kill switch'
                  : 'Salvar rollout'
            }}
          </AppPanelButton>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.ops-config {
  display: grid;
  gap: 1rem;
}
.ops-config__header,
.ops-config__actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}
.ops-config__header h3,
.ops-config__header p {
  margin: 0;
}
.ops-config__header p,
.ops-config__muted,
small {
  color: var(--text-muted);
}
.ops-config__summary,
.ops-config__legacy {
  display: grid;
  gap: 0.35rem;
  padding: 0.85rem;
  border: 1px solid var(--border-subtle);
  border-radius: 0.75rem;
}
.ops-config__summary.is-degraded {
  border-color: rgb(var(--warning) / 0.6);
}
.ops-config__alerts,
.ops-config__form,
.ops-config__schedule {
  display: grid;
  gap: 0.75rem;
}
.ops-config__alerts article {
  display: grid;
  gap: 0.25rem;
  padding: 0.75rem;
  border-left: 3px solid rgb(var(--warning));
  background: rgb(var(--surface-raised));
}
.ops-config__alerts article.is-critical {
  border-left-color: rgb(var(--error));
}
.ops-config__form label {
  display: grid;
  gap: 0.35rem;
}
.ops-config__form input,
.ops-config__form select,
.ops-config__form textarea {
  width: 100%;
  padding: 0.65rem;
  border: 1px solid var(--border-subtle);
  border-radius: 0.55rem;
  background: var(--surface-base);
  color: var(--text-primary);
}
.ops-config__grid,
.ops-config__window {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.ops-config__window {
  grid-template-columns: 1.5fr 1fr 1fr auto;
}
.ops-config fieldset {
  display: grid;
  gap: 0.45rem;
  border: 1px solid var(--border-subtle);
  border-radius: 0.65rem;
}
.ops-config__check {
  display: flex !important;
  grid-template-columns: auto 1fr;
  align-items: center;
}
.ops-config__check input {
  width: auto;
}
@media (max-width: 720px) {
  .ops-config__grid,
  .ops-config__window {
    grid-template-columns: 1fr;
  }
}
</style>
