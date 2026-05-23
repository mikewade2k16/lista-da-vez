<script setup>
import { computed } from 'vue'

import { getRoleLabel } from '~/domain/utils/user-access'

const props = defineProps({
  compact: {
    type: Boolean,
    default: false,
  },
  role: {
    type: String,
    default: '',
  },
})

const label = computed(() => getRoleLabel(props.role))
</script>

<template>
  <span class="users-access-manager__role-badge" :class="{ 'is-compact': compact }">
    {{ label }}
  </span>
</template>
