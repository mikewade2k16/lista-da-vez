<script setup lang="ts">
import { useMetaAdsStore } from '~/stores/meta-ads'

const store = useMetaAdsStore()

function onSelect(id: string) {
  if (id === store.selectedAdAccountId || store.pending) return
  void store.selectAdAccount(id)
}
</script>

<template>
  <section v-if="store.adAccounts.length" class="ma-picker" aria-label="Contas de anuncio">
    <p class="ma-picker__head">Conta de anuncio</p>
    <div class="ma-picker__list" role="tablist">
      <button
        v-for="adAccount in store.adAccounts"
        :key="adAccount.id"
        type="button"
        role="tab"
        class="ma-picker__item"
        :class="{ 'ma-picker__item--active': adAccount.id === store.selectedAdAccountId }"
        :aria-selected="adAccount.id === store.selectedAdAccountId"
        :disabled="store.pending"
        @click="onSelect(adAccount.id)"
      >
        <span class="ma-picker__name">{{ adAccount.name || adAccount.metaAdAccountId }}</span>
        <span v-if="adAccount.currency" class="ma-picker__currency">{{ adAccount.currency }}</span>
      </button>
    </div>
  </section>
</template>

<style scoped>
.ma-picker {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.ma-picker__head {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.ma-picker__list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.ma-picker__item {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.9rem;
  border-radius: var(--radius-soft);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface) / 0.7);
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.88rem;
  font-weight: 500;
  transition:
    background 0.12s ease,
    color 0.12s ease,
    border-color 0.12s ease;
}

.ma-picker__item:hover:not(:disabled) {
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
}

.ma-picker__item--active {
  background: rgb(var(--primary) / 0.15);
  border-color: rgb(var(--primary) / 0.4);
  color: var(--text-main);
  font-weight: 600;
}

.ma-picker__item:disabled {
  opacity: 0.6;
  cursor: progress;
}

.ma-picker__name {
  min-width: 0;
}

.ma-picker__currency {
  font-size: 0.72rem;
  font-weight: 700;
  padding: 0.05rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.9);
  border: 1px solid var(--line-soft);
  color: var(--text-muted);
}

.ma-picker__item--active .ma-picker__currency {
  background: rgb(var(--primary) / 0.18);
  border-color: transparent;
  color: rgb(var(--primary));
}
</style>
