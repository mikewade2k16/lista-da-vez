<script setup lang="ts">
import { computed } from 'vue'
import AutomationHumanAttendanceGroup from './AutomationHumanAttendanceGroup.vue'
import type { AutomationAttendance } from '~/domain/omnichannel/automation-api'

const props = defineProps<{
  items: AutomationAttendance[]
  loading?: boolean
  resumingIds?: string[]
  pausingIds?: string[]
  replyingIds?: string[]
  closingIds?: string[]
}>()

defineEmits<{
  refresh: []
  resumeAi: [item: AutomationAttendance]
  pauseAi: [item: AutomationAttendance]
  replyAi: [item: AutomationAttendance]
  closeConversation: [item: AutomationAttendance]
}>()

const activeItems = computed(() => props.items.filter((item) => item.mode === 'ai_active'))
const stoppedItems = computed(() => props.items.filter((item) => item.mode === 'ai_stopped'))
const humanItems = computed(() => props.items.filter((item) => item.mode === 'human_active'))

const REASON_LABELS: Record<string, string> = {
  requested: 'Cliente pediu atendente',
  low_confidence: 'Confiança baixa',
  max_turns: 'Limite de conversa da IA',
  tool_failed: 'Consulta da IA falhou',
  policy: 'Automação interrompida',
  error: 'Falha na IA',
  model_handoff: 'IA solicitou apoio',
  operator_paused: 'IA pausada pelo operador',
}

function reasonLabel(reason: string): string {
  return REASON_LABELS[reason] || 'Intervenção necessária'
}

function reasonDescription(item: AutomationAttendance): string {
  if (
    item.reasonCode === 'low_confidence' &&
    item.aiConfidence !== null &&
    item.minimumConfidence !== null
  ) {
    return `A IA retornou ${Math.round(item.aiConfidence * 100)}% de confiança; o mínimo configurado para responder é ${Math.round(item.minimumConfidence * 100)}%.`
  }
  if (item.reasonCode === 'max_turns' && item.maxAiTurns !== null) {
    return `A conversa atingiu o máximo configurado de ${item.maxAiTurns} respostas automáticas.`
  }
  return item.summary
}

function activityLabel(value: string): string {
  const then = new Date(value).getTime()
  if (!Number.isFinite(then)) return 'agora'
  const minutes = Math.max(0, Math.floor((Date.now() - then) / 60_000))
  if (minutes < 1) return 'agora'
  if (minutes < 60) return `há ${minutes} min`
  return `há ${Math.floor(minutes / 60)}h`
}

function dispatchLabel(status: string): string {
  if (status === 'processing') return 'IA analisando agora'
  if (status === 'buffering' || status === 'queued') return 'Preparando resposta'
  return 'IA atendendo este contato'
}

function whatsappURL(phone: string): string {
  const digits = phone.replace(/\D/g, '')
  return digits ? `https://wa.me/${digits}` : 'https://web.whatsapp.com/'
}
</script>

<template>
  <section class="attendances">
    <header class="attendances__header">
      <div>
        <h2>Atendimentos da IA</h2>
        <p>Acompanhe a IA e aja sobre mensagens que ficaram sem resposta.</p>
      </div>
      <button type="button" class="card-action" :disabled="loading" @click="$emit('refresh')">
        <UIcon name="i-lucide-refresh-cw" />
        Atualizar
      </button>
    </header>

    <p v-if="loading" class="attendances__empty">Atualizando atendimentos…</p>

    <template v-else-if="items.length">
      <section v-if="activeItems.length" class="attendance-group">
        <header class="attendance-group__header">
          <div>
            <h3>IA atendendo</h3>
            <p>Conversas em que a automação continua responsável.</p>
          </div>
          <span>{{ activeItems.length }}</span>
        </header>

        <div class="attendance-grid">
          <article
            v-for="item in activeItems"
            :key="item.id"
            class="attendance-card attendance-card--active"
          >
            <header class="attendance-card__header">
              <span class="attendance-card__state attendance-card__state--active">
                <UIcon name="i-lucide-bot" />
                {{ dispatchLabel(item.dispatchStatus) }}
              </span>
              <time :datetime="item.activitySince">{{ activityLabel(item.activitySince) }}</time>
            </header>

            <div class="attendance-card__identity">
              <span class="attendance-card__avatar">
                {{ (item.contactName || 'C').slice(0, 1).toUpperCase() }}
              </span>
              <div>
                <strong>{{ item.contactName || 'Contato sem nome' }}</strong>
                <span>{{ item.contactPhone || item.instanceName }}</span>
              </div>
            </div>

            <div v-if="item.unansweredCount" class="attendance-card__pending">
              <strong>
                {{ item.unansweredCount }}
                {{ item.unansweredCount === 1 ? 'mensagem pendente' : 'mensagens pendentes' }}
              </strong>
              <p>{{ item.pendingMessagePreview }}</p>
            </div>

            <div class="attendance-card__meta">
              <span>{{ item.client.name }}</span>
              <span>{{ item.instanceName }}</span>
            </div>

            <div class="attendance-card__actions">
              <button
                type="button"
                class="card-action card-action--pause"
                :disabled="pausingIds?.includes(item.id)"
                @click="$emit('pauseAi', item)"
              >
                <UIcon
                  :name="
                    pausingIds?.includes(item.id) ? 'i-lucide-loader-circle' : 'i-lucide-pause'
                  "
                />
                {{ pausingIds?.includes(item.id) ? 'Pausando…' : 'Parar IA' }}
              </button>
              <a
                class="card-action"
                :href="whatsappURL(item.contactPhone)"
                target="_blank"
                rel="noopener noreferrer"
              >
                Abrir no WhatsApp
                <UIcon name="i-lucide-external-link" />
              </a>
            </div>
          </article>
        </div>
      </section>

      <AutomationHumanAttendanceGroup
        :items="humanItems"
        :resuming-ids="resumingIds"
        :replying-ids="replyingIds"
        :closing-ids="closingIds"
        @resume-ai="$emit('resumeAi', $event)"
        @reply-ai="$emit('replyAi', $event)"
        @close-conversation="$emit('closeConversation', $event)"
      />

      <section v-if="stoppedItems.length" class="attendance-group">
        <header class="attendance-group__header">
          <div>
            <h3>IA parada</h3>
            <p>Revise o motivo e responda agora as mensagens pendentes.</p>
          </div>
          <span>{{ stoppedItems.length }}</span>
        </header>

        <div class="attendance-grid">
          <article v-for="item in stoppedItems" :key="item.id" class="attendance-card">
            <header class="attendance-card__header">
              <span class="attendance-card__state">{{ reasonLabel(item.reasonCode) }}</span>
              <time :datetime="item.activitySince">{{ activityLabel(item.activitySince) }}</time>
            </header>

            <div class="attendance-card__identity">
              <span class="attendance-card__avatar">
                {{ (item.contactName || 'C').slice(0, 1).toUpperCase() }}
              </span>
              <div>
                <strong>{{ item.contactName || 'Contato sem nome' }}</strong>
                <span>{{ item.contactPhone || item.instanceName }}</span>
              </div>
            </div>

            <p v-if="reasonDescription(item)" class="attendance-card__summary">
              {{ reasonDescription(item) }}
            </p>

            <div v-if="item.unansweredCount" class="attendance-card__pending">
              <strong>
                {{ item.unansweredCount }}
                {{
                  item.unansweredCount === 1 ? 'mensagem sem resposta' : 'mensagens sem resposta'
                }}
              </strong>
              <p>{{ item.pendingMessagePreview }}</p>
            </div>

            <div class="attendance-card__meta">
              <span>{{ item.client.name }}</span>
              <span>{{ item.instanceName }}</span>
            </div>

            <div class="attendance-card__actions">
              <button
                v-if="item.unansweredCount > 0"
                type="button"
                class="card-action card-action--reply card-action--icon-only"
                aria-label="Forçar a IA a responder"
                :disabled="replyingIds?.includes(item.id)"
                title="Ordem manual: gera uma resposta mesmo após confiança baixa, máximo de respostas ou sugestão de transferência."
                @click="$emit('replyAi', item)"
              >
                <UIcon
                  :name="replyingIds?.includes(item.id) ? 'i-lucide-loader-circle' : 'i-lucide-bot'"
                />
                {{
                  replyingIds?.includes(item.id) ? 'Forçando resposta…' : 'Forçar IA a responder'
                }}
              </button>
              <button
                v-else
                type="button"
                class="card-action card-action--reply card-action--icon-only"
                aria-label="Retomar atendimento da IA nas próximas mensagens"
                :disabled="resumingIds?.includes(item.id)"
                title="A IA volta a atender quando o contato enviar uma nova mensagem."
                @click="$emit('resumeAi', item)"
              >
                <UIcon
                  :name="
                    resumingIds?.includes(item.id)
                      ? 'i-lucide-loader-circle'
                      : 'i-lucide-rotate-ccw'
                  "
                />
                {{ resumingIds?.includes(item.id) ? 'Retomando…' : 'Retomar nas próximas' }}
              </button>
              <button
                type="button"
                class="card-action card-action--close card-action--icon-only"
                aria-label="Encerrar conversa"
                title="Encerra esta conversa sem pedir uma resposta da IA."
                :disabled="closingIds?.includes(item.id)"
                @click="$emit('closeConversation', item)"
              >
                <UIcon
                  :name="
                    closingIds?.includes(item.id) ? 'i-lucide-loader-circle' : 'i-lucide-circle-x'
                  "
                />
                Encerrar conversa
              </button>
              <a
                class="card-action card-action--icon-only"
                aria-label="Abrir conversa no WhatsApp"
                title="Abrir conversa no WhatsApp"
                :href="whatsappURL(item.contactPhone)"
                target="_blank"
                rel="noopener noreferrer"
              >
                Abrir no WhatsApp
                <UIcon name="i-lucide-external-link" />
              </a>
            </div>
          </article>
        </div>
      </section>
    </template>

    <p v-else class="attendances__empty">
      Nenhum atendimento ativo ou aguardando intervenção para este cliente.
    </p>
  </section>
</template>

<style scoped>
.attendances,
.attendance-group {
  display: grid;
  gap: 0.85rem;
}

.attendances__header,
.attendance-group__header,
.attendance-card__header,
.attendance-card__identity,
.attendance-card__meta,
.attendance-card__actions {
  display: flex;
  align-items: center;
}

.attendances__header,
.attendance-group__header,
.attendance-card__header {
  justify-content: space-between;
  gap: 0.75rem;
}

.attendances__header h2,
.attendances__header p,
.attendance-group__header h3,
.attendance-group__header p,
.attendance-card__summary,
.attendance-card__pending p {
  margin: 0;
}

.attendances__header h2,
.attendance-group__header h3 {
  color: var(--text-main);
  font-size: 1rem;
}

.attendances__header p,
.attendance-group__header p,
.attendances__empty,
.attendance-card time,
.attendance-card__identity span,
.attendance-card__meta {
  color: var(--text-muted);
  font-size: 0.75rem;
}

.attendance-group {
  padding-top: 0.2rem;
}

.attendance-group + .attendance-group {
  padding-top: 1rem;
  border-top: 1px solid var(--line-soft);
}

.attendance-group__header > span {
  min-width: 24px;
  padding: 0.15rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 700;
  text-align: center;
}

.attendance-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(290px, 1fr));
  gap: 0.75rem;
}

.attendance-card {
  display: grid;
  gap: 0.75rem;
  padding: 0.9rem;
  border: 1px solid color-mix(in srgb, var(--accent-warning) 42%, var(--line-soft));
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.78);
}

.attendance-card--active {
  border-color: color-mix(in srgb, rgb(var(--success)) 42%, var(--line-soft));
}

.attendance-card__state {
  color: var(--accent-warning);
  font-size: 0.72rem;
  font-weight: 700;
}

.attendance-card__state--active {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: rgb(var(--success));
}

.attendance-card__identity {
  gap: 0.65rem;
}

.attendance-card__identity > div {
  display: grid;
}

.attendance-card__identity strong,
.attendance-card__summary,
.attendance-card__pending strong {
  color: var(--text-main);
  font-size: 0.82rem;
}

.attendance-card__avatar {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
  font-weight: 700;
}

.attendance-card__pending {
  display: grid;
  gap: 0.25rem;
  padding: 0.65rem;
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.72);
}

.attendance-card__pending p {
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.4;
}

.attendance-card__meta {
  flex-wrap: wrap;
  gap: 0.35rem;
}

.attendance-card__meta span {
  padding: 0.15rem 0.4rem;
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.7);
}

.attendance-card__actions {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(0, 1fr));
  gap: 0.45rem;
}

.card-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  min-height: 34px;
  padding: 0 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: var(--text-main);
  font: inherit;
  font-size: 0.76rem;
  text-decoration: none;
  cursor: pointer;
}

.card-action:disabled {
  cursor: wait;
  opacity: 0.65;
}

.card-action--reply {
  border-color: rgb(var(--primary) / 0.45);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.card-action--close {
  border-color: rgb(var(--danger) / 0.4);
  color: rgb(var(--danger));
}

.card-action--icon-only {
  min-width: 0;
  padding-inline: 0.5rem;
  font-size: 0;
}

.card-action--icon-only :deep(svg),
.card-action--icon-only :deep(.iconify),
.card-action--icon-only :deep([class*='i-lucide-']) {
  display: block;
  width: 1rem;
  height: 1rem;
  flex: 0 0 auto;
  font-size: 1rem !important;
  line-height: 1;
}

.card-action--pause {
  border-color: color-mix(in srgb, var(--accent-warning) 45%, var(--line-soft));
  color: var(--accent-warning);
}
</style>
