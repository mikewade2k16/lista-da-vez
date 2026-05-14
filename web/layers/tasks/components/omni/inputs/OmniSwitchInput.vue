<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    modelValue?: boolean
    disabled?: boolean
    loading?: boolean
    ariaLabel?: string
    checkedIcon?: string
    uncheckedIcon?: string
  }>(),
  {
    modelValue: false,
    disabled: false,
    loading: false,
    ariaLabel: 'Alternar',
    checkedIcon: '',
    uncheckedIcon: ''
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const switchUi = {
  root: 'omni-switch__root justify-center',
  container: 'omni-switch__container',
  base: 'omni-switch__base',
  thumb: 'omni-switch__thumb',
  icon: 'omni-switch__icon',
  wrapper: 'omni-switch__wrapper',
  label: 'omni-switch__label',
  description: 'omni-switch__description'
}

function onUpdate(value: boolean | 'indeterminate') {
  emit('update:modelValue', value === true)
}
</script>

<template>
  <USwitch
    v-bind="$attrs"
    class="omni-switch"
    :model-value="Boolean(props.modelValue)"
    :disabled="props.disabled"
    :loading="props.loading"
    :checked-icon="props.checkedIcon || undefined"
    :unchecked-icon="props.uncheckedIcon || undefined"
    :aria-label="props.ariaLabel"
    :ui="switchUi"
    @update:model-value="onUpdate"
  />
</template>
