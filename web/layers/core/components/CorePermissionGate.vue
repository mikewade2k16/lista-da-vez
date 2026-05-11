<script setup lang="ts">
import { computed } from "vue";
import { usePermission } from "../composables/usePermission";

const props = defineProps<{
  require?: string;
  requireAll?: string[];
  requireAny?: string[];
}>();

const { has, hasAll, hasAny } = usePermission();

const allowed = computed(() => {
  if (props.require) return has(props.require);
  if (props.requireAll?.length) return hasAll(...props.requireAll);
  if (props.requireAny?.length) return hasAny(...props.requireAny);
  return true;
});
</script>

<template>
  <slot v-if="allowed" />
  <slot v-else name="fallback" />
</template>
