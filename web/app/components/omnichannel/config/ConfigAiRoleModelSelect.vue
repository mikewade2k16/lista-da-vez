<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { fetchAICredentialModels } from '~/domain/omnichannel/config-api'

const props = defineProps<{
  credentialId: string
  capability: 'response' | 'audio' | 'image' | 'video' | 'document'
  modelValue: string
  disabled?: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)
const models = ref<string[]>([])
const loading = ref(false)
const error = ref('')
let generation = 0

const selectedExists = computed(() => models.value.includes(props.modelValue))

async function load(): Promise<void> {
  const current = ++generation
  models.value = []
  error.value = ''
  if (!props.credentialId) return
  loading.value = true
  try {
    const result = await fetchAICredentialModels(api, props.credentialId, props.capability)
    if (current !== generation) return
    models.value = result
  } catch (cause) {
    if (current !== generation) return
    error.value = getApiErrorMessage(cause, 'Não foi possível listar os modelos desta chave.')
  } finally {
    if (current === generation) loading.value = false
  }
}

watch([() => props.credentialId, () => props.capability], () => void load(), { immediate: true })
</script>

<template>
  <label class="calendar-config__field">
    <span class="calendar-config__field-label">Modelo</span>
    <select
      class="calendar-config__input"
      :value="modelValue"
      :disabled="disabled || loading || !credentialId"
      @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
    >
      <option v-if="!selectedExists" :value="modelValue" :disabled="!modelValue">
        {{ modelValue || (loading ? 'Carregando modelos…' : 'Selecione um modelo') }}
      </option>
      <option v-for="model in models" :key="model" :value="model">{{ model }}</option>
    </select>
    <span v-if="error" class="calendar-config__warn">{{ error }}</span>
    <span v-else-if="!loading && credentialId && models.length === 0" class="calendar-config__hint">
      Nenhum modelo compatível foi listado; confira o provedor escolhido para esta função.
    </span>
  </label>
</template>
