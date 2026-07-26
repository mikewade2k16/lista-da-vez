<script setup>
defineProps({
  tabs: {
    type: Array,
    default: () => [],
  },
  activeTab: {
    type: String,
    required: true,
  },
})

defineEmits(['update:activeTab'])
</script>

<template>
  <nav class="settings-tabs">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      :data-testid="`settings-tab-${tab.id}`"
      :class="['settings-tabs__btn', { 'is-active': activeTab === tab.id }]"
      @click="$emit('update:activeTab', tab.id)"
    >
      <span class="material-icons-round">{{ tab.icon }}</span>
      <span>{{ tab.label }}</span>
    </button>
  </nav>
</template>

<style scoped>
.settings-tabs {
  width: 100%;
  flex-wrap: nowrap;
  gap: 3px;
  padding: 2px;
  border-radius: 12px;
  overflow-x: auto;
  scrollbar-width: thin;
}

.settings-tabs__btn {
  gap: 5px;
  min-height: 32px;
  padding: 4px 10px;
  border-radius: 9px;
  font-size: 0.72rem;
  line-height: 1;
}

.settings-tabs__btn .material-icons-round {
  font-size: 14px;
}

@media (max-width: 760px) {
  .settings-tabs__btn {
    min-height: 30px;
    padding-inline: 8px;
  }
}
</style>
