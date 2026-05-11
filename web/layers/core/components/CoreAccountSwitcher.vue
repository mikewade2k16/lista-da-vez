<script setup lang="ts">
import { ref, computed } from "vue";
import { useCoreAccountStore } from "../stores/account";

const accountStore = useCoreAccountStore();
const open = ref(false);

const activeAccount = computed(() => accountStore.activeAccount);
const accounts = computed(() => accountStore.accounts);

async function select(id: string) {
  open.value = false;
  if (id !== accountStore.activeAccountId) {
    await accountStore.switchAccount(id);
  }
}

function toggle() {
  open.value = !open.value;
}
</script>

<template>
  <div class="core-account-switcher" :class="{ 'is-open': open }">
    <button
      class="core-account-switcher__trigger"
      type="button"
      :aria-expanded="open ? 'true' : 'false'"
      aria-haspopup="listbox"
      @click="toggle"
    >
      <span class="core-account-switcher__name">
        {{ activeAccount?.name ?? "Selecionar account" }}
      </span>
      <span class="core-account-switcher__arrow" aria-hidden="true">▾</span>
    </button>

    <ul
      v-if="open && accounts.length > 1"
      class="core-account-switcher__list"
      role="listbox"
      :aria-label="'Selecionar account'"
    >
      <li
        v-for="account in accounts"
        :key="account.id"
        class="core-account-switcher__option"
        :class="{ 'is-active': account.id === accountStore.activeAccountId }"
        role="option"
        :aria-selected="account.id === accountStore.activeAccountId ? 'true' : 'false'"
        tabindex="0"
        @click="select(account.id)"
        @keydown.enter="select(account.id)"
      >
        {{ account.name }}
      </li>
    </ul>
  </div>
</template>

<style scoped>
.core-account-switcher {
  position: relative;
}

.core-account-switcher__trigger {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.7rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  color: #e2e8f0;
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.core-account-switcher__trigger:hover {
  border-color: rgba(129, 140, 248, 0.32);
  background: rgba(129, 140, 248, 0.09);
}

.core-account-switcher__list {
  position: absolute;
  top: calc(100% + 0.35rem);
  left: 0;
  min-width: 12rem;
  margin: 0;
  padding: 0.35rem;
  list-style: none;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 10px;
  background: rgba(13, 18, 29, 0.97);
  box-shadow: 0 12px 28px rgba(2, 6, 23, 0.36);
  z-index: 100;
}

.core-account-switcher__option {
  padding: 0.5rem 0.65rem;
  border-radius: 7px;
  color: rgba(226, 232, 240, 0.82);
  font-size: 0.8rem;
  cursor: pointer;
  transition: background 0.13s ease, color 0.13s ease;
}

.core-account-switcher__option:hover,
.core-account-switcher__option:focus {
  background: rgba(129, 140, 248, 0.12);
  color: #eef2ff;
  outline: none;
}

.core-account-switcher__option.is-active {
  color: #c7d2fe;
  font-weight: 700;
}
</style>
