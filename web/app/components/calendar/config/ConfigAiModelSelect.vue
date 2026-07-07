<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'
import { fetchAiModels } from '~/domain/calendar/calendar-api'
import { AI_PROVIDER_LABEL, type CalendarAiProvider } from '~/utils/calendar'

// Modelo como SELECT (Opcao C): ao inves de texto livre, o campo lista os modelos REAIS
// do provedor (buscados pelo back, que resolve a chave server-side). Isso elimina a
// armadilha provider=OpenAI + model=gemini-*. Decisao do usuario: SELECT sempre, sem
// digitacao a mao; quando nao da pra listar (sem chave / provedor sem /models / API
// falhou) o campo fica DESABILITADO com aviso do motivo + botao "Tentar novamente".
const props = defineProps<{ provider: CalendarAiProvider; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

const models = ref<string[]>([])
const loading = ref(false)
const errorMsg = ref('')
// Contador de geracao: ignora respostas de uma busca antiga quando o provedor muda no
// meio do caminho (evita popular o select com a lista do provedor errado).
let reqGen = 0

const providerLabel = computed(() => AI_PROVIDER_LABEL[props.provider] || props.provider)
// O valor salvo pode nao estar na lista (provedor recem-trocado ou modelo descontinuado):
// nesse caso mostramos o placeholder e nao "fingimos" que ha selecao valida.
const hasValidSelection = computed(
  () => props.modelValue !== '' && models.value.includes(props.modelValue),
)

// errorCodeOf le o codigo do erro padronizado do back ({error:{code}}) sem usar `any`.
function errorCodeOf(err: unknown): string {
  const code = (err as { data?: { error?: { code?: unknown } } })?.data?.error?.code
  return typeof code === 'string' ? code : ''
}

function messageForCode(code: string): string {
  switch (code) {
    case 'ai_key_missing':
      return 'Configure a chave deste provedor no bloco "Chaves de API" acima e clique em Tentar novamente.'
    case 'models_unavailable':
      return 'Não foi possível listar os modelos (a API do provedor falhou ou a chave é inválida). Verifique a chave e tente novamente.'
    case 'invalid_provider':
      return 'Este provedor não oferece listagem de modelos.'
    default:
      return 'Falha ao buscar os modelos. Tente novamente.'
  }
}

async function load(): Promise<void> {
  const gen = ++reqGen
  loading.value = true
  errorMsg.value = ''
  models.value = []
  try {
    const list = await fetchAiModels(apiRequest, props.provider)
    if (gen !== reqGen) return
    models.value = list
    // Se o modelo salvo nao existe na lista do provedor atual, limpa (fonte unica: nao
    // deixa persistir um modelo invalido para o provedor selecionado — o usuario escolhe
    // um valido). So limpa em busca BEM-SUCEDIDA; erro preserva o valor salvo.
    if (props.modelValue !== '' && !list.includes(props.modelValue)) {
      emit('update:modelValue', '')
    }
  } catch (err) {
    if (gen !== reqGen) return
    errorMsg.value = messageForCode(errorCodeOf(err))
  } finally {
    if (gen === reqGen) loading.value = false
  }
}

function onPick(value: string): void {
  emit('update:modelValue', value)
}

onMounted(() => void load())
// Trocar o provedor recarrega a lista (e limpa o modelo antigo se nao pertencer ao novo).
watch(
  () => props.provider,
  () => void load(),
)
</script>

<template>
  <div class="calendar-config__field">
    <span class="calendar-config__field-label">Modelo</span>
    <select
      class="calendar-config__input"
      :value="modelValue"
      :disabled="loading || errorMsg !== '' || models.length === 0"
      @change="onPick(($event.target as HTMLSelectElement).value)"
    >
      <option v-if="!hasValidSelection" value="" disabled>
        {{ loading ? 'Carregando modelos…' : 'Selecione um modelo' }}
      </option>
      <option v-for="m in models" :key="m" :value="m">{{ m }}</option>
    </select>

    <span v-if="loading" class="calendar-config__hint">Buscando modelos do provedor…</span>
    <template v-else-if="errorMsg">
      <span class="calendar-config__warn">
        <UIcon name="i-lucide-triangle-alert" aria-hidden="true" />
        {{ errorMsg }}
      </span>
      <span v-if="modelValue" class="calendar-config__hint">Modelo salvo: {{ modelValue }}</span>
      <AppPanelButton variant="ghost" @click="load">Tentar novamente</AppPanelButton>
    </template>
    <span v-else class="calendar-config__hint">
      {{ models.length }} modelo(s) de {{ providerLabel }}. Trocar o provedor recarrega a lista.
    </span>
  </div>
</template>
