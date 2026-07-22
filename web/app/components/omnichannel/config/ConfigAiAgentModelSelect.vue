<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { fetchAgentModels } from '~/domain/omnichannel/ai-models-api'
import { AI_PROVIDER_LABEL, type CalendarAiProvider } from '~/utils/calendar'

const props = defineProps<{
  agentId: string
  provider: CalendarAiProvider
  modelValue: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)
const models = ref<string[]>([])
const loading = ref(false)
const errorMessage = ref('')
let requestGeneration = 0

const providerLabel = computed(() => AI_PROVIDER_LABEL[props.provider] || props.provider)
const hasValidSelection = computed(
  () => props.modelValue !== '' && models.value.includes(props.modelValue),
)

async function load(): Promise<void> {
  const generation = ++requestGeneration
  loading.value = true
  errorMessage.value = ''
  models.value = []
  try {
    const result = await fetchAgentModels(api, props.agentId, props.provider)
    if (generation !== requestGeneration) return
    models.value = result
    if (props.modelValue && !result.includes(props.modelValue)) {
      emit('update:modelValue', '')
    }
  } catch (error) {
    if (generation !== requestGeneration) return
    errorMessage.value = getApiErrorMessage(
      error,
      'Não foi possível listar os modelos. Salve a chave da API e tente novamente.',
    )
  } finally {
    if (generation === requestGeneration) loading.value = false
  }
}

watch([() => props.agentId, () => props.provider], () => void load(), { immediate: true })
</script>

<template>
  <div class="calendar-config__field">
    <span class="calendar-config__field-label">Modelo</span>
    <select
      class="calendar-config__input"
      :value="modelValue"
      :disabled="disabled || loading || !!errorMessage || models.length === 0"
      @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
    >
      <option v-if="!hasValidSelection" value="" disabled>
        {{ loading ? 'Carregando modelos…' : 'Selecione um modelo' }}
      </option>
      <option v-for="model in models" :key="model" :value="model">{{ model }}</option>
    </select>

    <span v-if="loading" class="calendar-config__hint">
      Buscando modelos de {{ providerLabel }}…
    </span>
    <template v-else-if="errorMessage">
      <span class="calendar-config__warn">
        <UIcon name="i-lucide-triangle-alert" aria-hidden="true" />
        {{ errorMessage }}
      </span>
      <AppPanelButton variant="ghost" :disabled="disabled" @click="load">
        Tentar novamente
      </AppPanelButton>
    </template>
  </div>
</template>
