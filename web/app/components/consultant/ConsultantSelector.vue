<script setup>
defineProps({
  roster: {
    type: Array,
    default: () => []
  },
  selectedConsultantId: {
    type: String,
    default: ""
  }
});

const emit = defineEmits(["select"]);
</script>

<template>
  <div class="admin-selector" data-testid="consultant-selector">
    <button
      v-for="consultant in roster"
      :key="consultant.id"
      type="button"
      class="admin-selector__button"
      :class="{ 'admin-selector__button--active': consultant.id === selectedConsultantId }"
      @click="emit('select', consultant.id)"
    >
      {{ consultant.name }}
    </button>
  </div>
</template>
