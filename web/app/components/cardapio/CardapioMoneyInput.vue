<script setup lang="ts">
import OmniDecimalMoneyInput from '../../../layers/tasks/components/omni/inputs/OmniDecimalMoneyInput.vue'

// O modelo do cardapio guarda centavos; o editor recebe reais.
defineProps<{
  modelValue: number
  placeholder?: string
  disabled?: boolean
  wholeAmounts?: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [value: number] }>()
</script>

<template>
  <OmniDecimalMoneyInput
    :model-value="modelValue / 100"
    :whole-amounts="wholeAmounts"
    :placeholder="placeholder"
    :disabled="disabled"
    update-on-input
    @update:model-value="emit('update:modelValue', Math.round($event * 100))"
  />
</template>
