<script setup>
import { computed } from "vue";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const emit = defineEmits(["store-change", "profile-change"]);

const activeStore = computed(() =>
  (props.state.stores || []).find((store) => store.id === props.state.activeStoreId) || null
);
const activeServicesCount = computed(() => props.state.activeServices?.length || 0);

function handleStoreChange(event) {
  emit("store-change", event.target.value);
}

function handleProfileChange(event) {
  emit("profile-change", event.target.value);
}
</script>

<template>
  <header class="app-header">
    <div class="brand-bar">
      <div class="brand">
        <span class="brand__name">{{ state.brandName }}</span>
        <span class="brand__sub">
          {{ state.pageTitle }}<template v-if="activeStore"> | {{ activeStore.name }}</template>
        </span>
      </div>
      <div class="brand__meta">
        <span class="summary-pill">{{ state.waitingList.length }} na fila</span>
        <span
          class="summary-pill"
          :class="{ 'summary-pill--active': activeServicesCount > 0 }"
        >
          {{ activeServicesCount }}/{{ state.settings.maxConcurrentServices }} em atendimento
        </span>
        <span class="summary-pill">{{ state.serviceHistory.length }} finalizados</span>
        <label class="summary-select">
          <span style="display: none;">Loja:</span>
          <select :value="state.activeStoreId" aria-label="Loja ativa" @change="handleStoreChange">
            <option
              v-for="store in state.stores"
              :key="store.id"
              :value="store.id"
            >
              {{ store.name }}
            </option>
          </select>
        </label>
        <label class="summary-select">
          <span style="display: none;">Perfil:</span>
          <select :value="state.activeProfileId" aria-label="Perfil de acesso" @change="handleProfileChange">
            <option
              v-for="profile in state.profiles"
              :key="profile.id"
              :value="profile.id"
            >
              {{ profile.name }}
            </option>
          </select>
        </label>
      </div>
    </div>
  </header>
</template>
