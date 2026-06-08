<script setup lang="ts">
import type { AccountItem } from '~/types/accounts'
import { accountCardFields } from './account-fields'

defineProps<{ account: AccountItem }>()
const emit = defineEmits<{ (e: 'open', account: AccountItem): void }>()

const cardFields = accountCardFields()
</script>

<template>
  <article
    class="account-card"
    :class="{ 'account-card--inactive': account.status !== 'active' }"
    @dblclick="emit('open', account)"
  >
    <header class="account-card__header">
      <div class="account-card__identity">
        <h3 class="account-card__name">{{ account.name || 'Sem nome' }}</h3>
        <UBadge
          :color="account.status === 'active' ? 'success' : 'neutral'"
          variant="soft"
          size="sm"
        >
          {{ account.status === 'active' ? 'Ativo' : 'Inativo' }}
        </UBadge>
      </div>
      <UButton
        icon="i-lucide-maximize-2"
        color="neutral"
        variant="ghost"
        size="sm"
        title="Abrir detalhes"
        aria-label="Abrir detalhes"
        @click="emit('open', account)"
      />
    </header>

    <dl class="account-card__fields">
      <div v-for="field in cardFields" :key="field.key" class="account-card__field">
        <dt class="account-card__field-label">{{ field.label }}</dt>
        <dd class="account-card__field-value">{{ field.display(account) }}</dd>
      </div>
    </dl>
  </article>
</template>

<style scoped>
.account-card {
  display: grid;
  gap: 0.75rem;
  padding: 0.95rem 1rem;
  border-radius: 1rem;
  border: 1px solid rgb(var(--border) / 0.18);
  background: rgb(var(--surface) / 0.6);
  cursor: default;
}

.account-card--inactive {
  opacity: 0.72;
}

.account-card__header {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 0.6rem;
}

.account-card__identity {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  min-width: 0;
}

.account-card__name {
  margin: 0;
  color: var(--text-main);
  font-size: 0.95rem;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-card__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: 0.5rem 0.85rem;
  margin: 0;
}

.account-card__field {
  display: grid;
  gap: 0.15rem;
  min-width: 0;
}

.account-card__field-label {
  color: var(--text-muted);
  font-size: 0.68rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.account-card__field-value {
  margin: 0;
  color: var(--text-main);
  font-size: 0.82rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
