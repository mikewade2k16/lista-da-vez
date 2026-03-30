<script setup>
defineProps({
  tabs: {
    type: Array,
    default: () => []
  },
  activeTab: {
    type: String,
    required: true
  }
});

defineEmits(["update:active-tab"]);
</script>

<template>
  <nav class="settings-tabs">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      :data-testid="`settings-tab-${tab.id}`"
      :class="['settings-tabs__btn', { 'is-active': activeTab === tab.id }]"
      @click="$emit('update:active-tab', tab.id)"
    >
      <span class="material-icons-round">{{ tab.icon }}</span>
      <span>{{ tab.label }}</span>
    </button>
  </nav>
</template>
