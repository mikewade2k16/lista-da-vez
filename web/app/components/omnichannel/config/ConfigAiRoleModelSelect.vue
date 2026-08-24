<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { fetchAICredentialModels } from '~/domain/omnichannel/config-api'

const props = defineProps<{
  credentialId: string
  capability: 'response' | 'audio' | 'image' | 'video' | 'document'
  modelValue: string
  disabled?: boolean
  accountId?: string
  credentialBasePath?: string
}>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)
const models = ref<string[]>([])
const loading = ref(false)
const error = ref('')
let generation = 0
let controller: AbortController | null = null

const selectedExists = computed(() => models.value.includes(props.modelValue))

async function load(): Promise<void> {
  const current = ++generation
  controller?.abort()
  const requestController = new AbortController()
  controller = requestController
  models.value = []
  error.value = ''
  if (!props.credentialId) {
    controller = null
    return
  }
  loading.value = true
  try {
    const accountId = String(props.accountId || '').trim()
    const basePath = String(props.credentialBasePath || '').trim()
    const result = await fetchAICredentialModels(api, props.credentialId, props.capability, {
      ...(accountId ? { headers: { 'X-Account-Id': accountId } } : {}),
      ...(basePath ? { basePath } : {}),
      signal: requestController.signal,
    })
    if (current !== generation) return
    models.value = result
  } catch (cause) {
    if (current !== generation) return
    error.value = getApiErrorMessage(cause, 'Não foi possível listar os modelos desta chave.')
  } finally {
    if (current === generation) {
      controller = null
      loading.value = false
    }
  }
}

watch(
  [
    () => props.credentialId,
    () => props.capability,
    () => props.accountId,
    () => props.credentialBasePath,
  ],
  () => void load(),
  { immediate: true, flush: 'sync' },
)

onScopeDispose(() => controller?.abort())
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
