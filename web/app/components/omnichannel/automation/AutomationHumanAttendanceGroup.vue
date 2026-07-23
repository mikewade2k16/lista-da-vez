<script setup lang="ts">
import type { AutomationAttendance } from '~/domain/omnichannel/automation-api'

defineProps<{
  items: AutomationAttendance[]
  resumingIds?: string[]
  replyingIds?: string[]
  closingIds?: string[]
}>()

defineEmits<{
  resumeAi: [item: AutomationAttendance]
  replyAi: [item: AutomationAttendance]
  closeConversation: [item: AutomationAttendance]
}>()

function activityLabel(value: string): string {
  const then = new Date(value).getTime()
  if (!Number.isFinite(then)) return 'agora'
  const minutes = Math.max(0, Math.floor((Date.now() - then) / 60_000))
  if (minutes < 1) return 'agora'
  if (minutes < 60) return `há ${minutes} min`
  return `há ${Math.floor(minutes / 60)}h`
}

function whatsappURL(phone: string): string {
  const digits = phone.replace(/\D/g, '')
  return digits ? `https://wa.me/${digits}` : 'https://web.whatsapp.com/'
}
</script>

<template>
  <section v-if="items.length" class="attendance-group">
    <header class="attendance-group__header">
      <div>
        <h3>Atendimento humano</h3>
        <p>A IA não responde enquanto a conversa permanecer sob controle humano.</p>
      </div>
      <span>{{ items.length }}</span>
    </header>

    <div class="attendance-grid">
      <article v-for="item in items" :key="item.id" class="attendance-card">
        <header class="attendance-card__header">
          <span class="attendance-card__state">Atendimento humano ativo</span>
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
            v-if="item.unansweredCount > 0"
            type="button"
            class="card-action card-action--reply card-action--icon-only"
            aria-label="Passar esta conversa para a IA agora"
            :disabled="replyingIds?.includes(item.id)"
            title="Transfere a conversa para a IA e responde as mensagens pendentes."
            @click="$emit('replyAi', item)"
          >
            <UIcon
              :name="replyingIds?.includes(item.id) ? 'i-lucide-loader-circle' : 'i-lucide-bot'"
            />
            {{ replyingIds?.includes(item.id) ? 'Transferindo…' : 'Passar para IA agora' }}
          </button>
          <button
            v-else
            type="button"
            class="card-action card-action--reply card-action--icon-only"
            aria-label="Passar esta conversa para a IA nas próximas mensagens"
            :disabled="resumingIds?.includes(item.id)"
            title="Encerra o controle humano atual; a IA assume no próximo inbound."
            @click="$emit('resumeAi', item)"
          >
            <UIcon
              :name="resumingIds?.includes(item.id) ? 'i-lucide-loader-circle' : 'i-lucide-bot'"
            />
            {{ resumingIds?.includes(item.id) ? 'Transferindo…' : 'Passar para IA nas próximas' }}
          </button>
          <button
            type="button"
            class="card-action card-action--close card-action--icon-only"
            aria-label="Encerrar conversa"
            :disabled="closingIds?.includes(item.id)"
            title="Encerra esta conversa sem pedir uma resposta da IA."
            @click="$emit('closeConversation', item)"
          >
            <UIcon
              :name="closingIds?.includes(item.id) ? 'i-lucide-loader-circle' : 'i-lucide-circle-x'"
            />
            {{ closingIds?.includes(item.id) ? 'Encerrando…' : 'Encerrar conversa' }}
          </button>
          <a
            class="card-action card-action--icon-only"
            aria-label="Abrir conversa no WhatsApp"
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

<style scoped>
.attendance-group {
  display: grid;
  gap: 0.85rem;
  padding-top: 1rem;
  border-top: 1px solid var(--line-soft);
}

.attendance-group__header,
.attendance-card__header,
.attendance-card__identity,
.attendance-card__meta,
.attendance-card__actions {
  display: flex;
  align-items: center;
}

.attendance-group__header,
.attendance-card__header {
  justify-content: space-between;
  gap: 0.75rem;
}

.attendance-group__header h3,
.attendance-group__header p,
.attendance-card__pending p {
  margin: 0;
}

.attendance-group__header h3 {
  color: var(--text-main);
  font-size: 1rem;
}

.attendance-group__header p,
.attendance-card time,
.attendance-card__identity span,
.attendance-card__meta {
  color: var(--text-muted);
  font-size: 0.75rem;
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

.attendance-card__state {
  color: var(--accent-warning);
  font-size: 0.72rem;
  font-weight: 700;
}

.attendance-card__identity {
  gap: 0.65rem;
}

.attendance-card__identity > div,
.attendance-card__pending {
  display: grid;
}

.attendance-card__identity strong,
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
  grid-template-columns: repeat(3, minmax(0, 1fr));
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

.card-action--icon-only :deep(svg) {
  width: 1rem;
  height: 1rem;
  flex: 0 0 auto;
}

.card-action--icon-only :deep(.iconify) {
  display: block;
  width: 1rem;
  height: 1rem;
  flex: 0 0 auto;
  font-size: 1rem;
  line-height: 1;
}

.card-action--icon-only :deep([class*='i-lucide-']) {
  display: block;
  width: 1rem;
  height: 1rem;
  flex: 0 0 auto;
  font-size: 1rem !important;
}

@media (max-width: 360px) {
  .attendance-card__actions {
    gap: 0.3rem;
  }
}
</style>
