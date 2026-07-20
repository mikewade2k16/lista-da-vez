<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  connectSession,
  fetchQrCode,
  fetchSessionStatus,
  logoutSession,
} from '~/domain/omnichannel/config-api'
import type { OmniSession } from '~/domain/omnichannel/config-types'

// Conexao/QR do numero. Lê o estado de /status (traz o `provider` que a view de gestão
// não projeta) e o QR de /qrcode. Pareamento: "Conectar" chama /connect (devolve QR ou
// já pareado); "Atualizar" relê o QR; "Desconectar" faz logout.
const props = defineProps<{ instanceName: string; disabled?: boolean }>()
const emit = defineEmits<{ 'provider-resolved': [provider: string] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const session = ref<OmniSession | null>(null)
const busy = ref(false)
const loading = ref(false)

const connected = computed(() => Boolean(session.value?.connected))
const qrCode = computed(() => session.value?.qrCode || '')

function apply(next: OmniSession): void {
  session.value = next
  if (next.provider) emit('provider-resolved', next.provider)
}

async function refresh(): Promise<void> {
  loading.value = true
  try {
    apply(await fetchSessionStatus(api, props.instanceName))
  } catch {
    // Instância sem sessão iniciada ainda (404): estado desconhecido, sem toast ruidoso.
    session.value = null
  } finally {
    loading.value = false
  }
}

async function connect(): Promise<void> {
  busy.value = true
  try {
    apply(await connectSession(api, props.instanceName))
    if (!session.value?.qrCode && !session.value?.connected) {
      // Alguns providers empurram o QR por webhook — relê do cache.
      apply(await fetchQrCode(api, props.instanceName))
    }
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível iniciar a conexão.'))
  } finally {
    busy.value = false
  }
}

async function disconnect(): Promise<void> {
  busy.value = true
  try {
    apply(await logoutSession(api, props.instanceName))
    ui.success('Número desconectado.')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível desconectar.'))
  } finally {
    busy.value = false
  }
}

onMounted(() => void refresh())
watch(
  () => props.instanceName,
  () => void refresh(),
)
</script>

<template>
  <div class="cfg-conn">
    <div class="cfg-conn__status">
      <span class="cfg-conn__dot" :class="connected ? 'is-on' : 'is-off'" aria-hidden="true"></span>
      <span class="cfg-conn__label">
        <template v-if="loading">Verificando…</template>
        <template v-else-if="connected">
          Conectado
          <template v-if="session?.phoneNumber">— {{ session.phoneNumber }}</template>
        </template>
        <template v-else>Desconectado</template>
      </span>
    </div>

    <div v-if="qrCode && !connected" class="cfg-conn__qr">
      <img :src="qrCode" alt="QR code para parear o WhatsApp" class="cfg-conn__qr-img" />
      <p class="cfg-conn__qr-hint">
        Abra o WhatsApp no celular do número → Aparelhos conectados → escaneie o QR.
      </p>
    </div>

    <div class="cfg-conn__actions">
      <AppPanelButton variant="ghost" :disabled="disabled || busy" @click="refresh">
        Atualizar
      </AppPanelButton>
      <AppPanelButton
        v-if="!connected"
        variant="secondary"
        :disabled="disabled || busy"
        @click="connect"
      >
        Conectar (gerar QR)
      </AppPanelButton>
      <AppPanelButton v-else variant="ghost" :disabled="disabled || busy" @click="disconnect">
        Desconectar
      </AppPanelButton>
    </div>
  </div>
</template>

<style scoped>
.cfg-conn {
  display: grid;
  gap: 0.6rem;
}

.cfg-conn__status {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.82rem;
  color: rgb(var(--text));
}

.cfg-conn__dot {
  width: 0.6rem;
  height: 0.6rem;
  border-radius: 999px;
}

.cfg-conn__dot.is-on {
  background: rgb(var(--success));
}

.cfg-conn__dot.is-off {
  background: rgb(var(--border));
}

.cfg-conn__qr {
  display: grid;
  gap: 0.4rem;
  justify-items: start;
}

.cfg-conn__qr-img {
  width: 200px;
  height: 200px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(255 255 255);
}

.cfg-conn__qr-hint {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.cfg-conn__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
</style>
