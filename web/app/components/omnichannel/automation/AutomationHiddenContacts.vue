<script setup lang="ts">
import type { HiddenOmnichannelContact } from '~/domain/omnichannel/privacy-api'

defineProps<{
  items: HiddenOmnichannelContact[]
  loading?: boolean
  restoringIds?: string[]
}>()

defineEmits<{
  refresh: []
  restore: [item: HiddenOmnichannelContact]
}>()

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 'agora' : date.toLocaleString('pt-BR')
}
</script>

<template>
  <section class="hidden-contacts">
    <header class="hidden-contacts__header">
      <div>
        <h2>Pessoas ocultas</h2>
        <p>Não aparecem no Omnichannel nem nos cards da Automação.</p>
      </div>
      <button
        type="button"
        class="hidden-contact__action"
        :disabled="loading"
        @click="$emit('refresh')"
      >
        <UIcon name="i-lucide-refresh-cw" />
        Atualizar
      </button>
    </header>

    <p v-if="loading" class="hidden-contacts__empty">Carregando pessoas ocultas…</p>
    <div v-else-if="items.length" class="hidden-contacts__grid">
      <article v-for="item in items" :key="item.contactId" class="hidden-contact">
        <div class="hidden-contact__identity">
          <span>{{ (item.contactName || 'C').slice(0, 1).toUpperCase() }}</span>
          <div>
            <strong>{{ item.contactName || 'Contato sem nome' }}</strong>
            <p>{{ item.contactPhone || 'Sem telefone' }}</p>
          </div>
        </div>
        <div class="hidden-contact__meta">
          <span>Oculto em {{ formatDate(item.hiddenAt) }}</span>
          <strong v-if="item.historyClearedAt">Histórico anterior limpo</strong>
          <span v-else>Histórico preservado</span>
        </div>
        <button
          type="button"
          class="hidden-contact__action hidden-contact__action--restore"
          :disabled="restoringIds?.includes(item.contactId)"
          @click="$emit('restore', item)"
        >
          <UIcon
            :name="
              restoringIds?.includes(item.contactId) ? 'i-lucide-loader-circle' : 'i-lucide-eye'
            "
          />
          {{ restoringIds?.includes(item.contactId) ? 'Restaurando…' : 'Voltar a exibir' }}
        </button>
      </article>
    </div>
    <p v-else class="hidden-contacts__empty">Nenhuma pessoa está oculta nesta conta.</p>
  </section>
</template>

<style scoped>
.hidden-contacts {
  display: grid;
  gap: 0.85rem;
}
.hidden-contacts__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}
.hidden-contacts__header h2,
.hidden-contacts__header p,
.hidden-contact p {
  margin: 0;
}
.hidden-contacts__header h2 {
  color: var(--text-main);
  font-size: 1rem;
}
.hidden-contacts__header p,
.hidden-contacts__empty,
.hidden-contact p,
.hidden-contact__meta {
  color: var(--text-muted);
  font-size: 0.75rem;
}
.hidden-contacts__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 0.75rem;
}
.hidden-contact {
  display: grid;
  gap: 0.75rem;
  padding: 0.9rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
}
.hidden-contact__identity {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}
.hidden-contact__identity > span {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
  font-weight: 800;
}
.hidden-contact__identity div,
.hidden-contact__meta {
  display: grid;
  gap: 0.15rem;
}
.hidden-contact__meta strong {
  color: rgb(var(--warning));
}
.hidden-contact__action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  min-height: 34px;
  padding: 0 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-main);
  font: inherit;
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
}
.hidden-contact__action--restore {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.45);
}
.hidden-contact__action:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
</style>
