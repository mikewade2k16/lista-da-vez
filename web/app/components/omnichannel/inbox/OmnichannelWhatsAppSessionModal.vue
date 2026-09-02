<script setup lang="ts">
import {
  UAlert,
  UBadge,
  UButton,
  UFormField,
  UInput,
  UModal,
  USelect
} from "#components";
import { computed, onBeforeUnmount, watch } from "vue";
import { useOmnichannelWhatsAppSession } from "~/composables/omnichannel/useOmnichannelWhatsAppSession";

const props = defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  (event: "update:open", value: boolean): void;
}>();

const {
  activate,
  canManageChannel,
  canResetHistory,
  clearConversationHistory,
  clearingHistory,
  connectionBadgeColor,
  connectionStateLabel,
  deactivate,
  disconnectSession,
  disconnecting,
  displayName,
  errorMessage,
  fetchingQr,
  generateQrCode,
  generatingQr,
  infoMessage,
  instanceItems,
  loadingInstances,
  persistDisplayName,
  qrImageSrc,
  qrUnavailableMessage,
  savingDisplayName,
  selectInstance,
  selectedInstance,
  selectedInstanceKey
} = useOmnichannelWhatsAppSession();

const openModel = computed({
  get: () => props.open,
  set: (value: boolean) => emit("update:open", value)
});

// "Conectado" desliga o CTA de QR (nao faz sentido gerar QR ja conectado).
const isConnected = computed(() => connectionStateLabel.value === "Conectado");
const isBusy = computed(() => generatingQr.value || fetchingQr.value || loadingInstances.value);

watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen) {
      await activate();
      return;
    }

    deactivate();
  },
  { immediate: true }
);

onBeforeUnmount(() => {
  deactivate();
});

async function handleClose() {
  openModel.value = false;
}

function normalizeSelectionValue(value: unknown) {
  if (typeof value === "string") {
    return value.trim();
  }

  if (value && typeof value === "object") {
    const record = value as Record<string, unknown>;
    if (typeof record.value === "string") {
      return record.value.trim();
    }
  }

  return "";
}

async function handleSelectionChange(value: unknown) {
  await selectInstance(normalizeSelectionValue(value));
}

async function handleDisplayNameBlur() {
  await persistDisplayName();
}

async function handleClearConversationHistory() {
  await clearConversationHistory();
}
</script>

<template>
  <UModal
    v-model:open="openModel"
    title="Conexao WhatsApp"
    :ui="{ content: 'max-w-lg' }"
  >
    <template #body>
      <UAlert
        v-if="!canManageChannel"
        color="warning"
        variant="soft"
        title="Sessao e QR restritos"
        description="A limpeza de historico aparece somente quando a API autoriza esta acao para a conexao."
      >
        <template #actions>
          <USelect
            v-if="instanceItems.length > 1"
            :model-value="selectedInstanceKey"
            :items="instanceItems"
            value-key="value"
            size="sm"
            class="wa-modal__select"
            :disabled="clearingHistory"
            @update:model-value="handleSelectionChange"
          />
        </template>
      </UAlert>

      <div v-else class="wa-modal">
        <!-- Status + qual conexao, numa linha -->
        <div class="wa-modal__head">
          <div class="wa-modal__id">
            <UBadge :color="connectionBadgeColor" variant="soft" size="sm">
              {{ connectionStateLabel }}
            </UBadge>
            <span class="wa-modal__name">
              {{ selectedInstance?.displayName || selectedInstance?.phoneNumber || "Nova conexao" }}
            </span>
          </div>
          <USelect
            v-if="instanceItems.length > 1"
            :model-value="selectedInstanceKey"
            :items="instanceItems"
            value-key="value"
            size="sm"
            class="wa-modal__select"
            :disabled="clearingHistory"
            @update:model-value="handleSelectionChange"
          />
        </div>

        <UAlert v-if="errorMessage" color="error" variant="soft" :title="errorMessage" />
        <UAlert v-if="infoMessage" color="success" variant="soft" :title="infoMessage" />

        <!-- QR em destaque: o coracao do modal -->
        <div class="wa-modal__qr">
          <template v-if="qrImageSrc">
            <img :src="qrImageSrc" alt="QR Code do WhatsApp" class="wa-modal__qr-img" />
            <p class="wa-modal__qr-hint">
              No celular: WhatsApp → <strong>Aparelhos conectados</strong> → <strong>Conectar aparelho</strong> → aponte a camera. O codigo renova sozinho.
            </p>
          </template>

          <template v-else-if="isConnected">
            <div class="wa-modal__qr-ok">✓</div>
            <p class="wa-modal__qr-hint">Sessao conectada. Nada a fazer aqui.</p>
          </template>

          <template v-else>
            <div class="wa-modal__qr-frame" :class="{ 'is-busy': isBusy }">
              <span v-if="isBusy" class="wa-modal__qr-frame-text">Gerando...</span>
              <span v-else class="wa-modal__qr-frame-text">QR</span>
            </div>
            <UButton
              color="primary"
              size="lg"
              block
              :loading="generatingQr"
              :disabled="isBusy"
              @click="generateQrCode()"
            >
              Gerar QR Code
            </UButton>
            <p class="wa-modal__qr-sub">{{ qrUnavailableMessage }}</p>
          </template>
        </div>

        <!-- Nome opcional, discreto -->
        <UFormField label="Nome da conexao (opcional)" name="displayName">
          <UInput
            v-model="displayName"
            placeholder="WhatsApp comercial"
            size="sm"
            :loading="savingDisplayName"
            @blur="handleDisplayNameBlur"
          />
        </UFormField>
      </div>
    </template>

    <template #footer>
      <div class="wa-modal__footer">
        <UButton color="neutral" variant="ghost" @click="handleClose">Fechar</UButton>
        <div v-if="canManageChannel || canResetHistory" class="wa-modal__footer-actions">
          <UButton
            v-if="canResetHistory"
            color="error"
            variant="ghost"
            size="sm"
            :loading="clearingHistory"
            :disabled="clearingHistory || disconnecting || !selectedInstance"
            @click="handleClearConversationHistory"
          >
            Limpar histórico visível desta conexão
          </UButton>
          <UButton
            v-if="canManageChannel"
            color="neutral"
            variant="outline"
            size="sm"
            :loading="disconnecting"
            :disabled="disconnecting || clearingHistory || !selectedInstance"
            @click="disconnectSession()"
          >
            Desconectar
          </UButton>
        </div>
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.wa-modal {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.wa-modal__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.wa-modal__id {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.wa-modal__name {
  font-size: 0.95rem;
  font-weight: 600;
  color: rgb(var(--text));
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.wa-modal__select {
  min-width: 12rem;
}

/* QR — centro do modal */
.wa-modal__qr {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.85rem;
  padding: 1.25rem 1rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.9rem;
  background: rgb(var(--surface-2));
  text-align: center;
}

.wa-modal__qr-img {
  width: min(100%, 240px);
  border-radius: 0.75rem;
  border: 1px solid rgb(var(--border));
  background: #fff;
  padding: 0.65rem;
}

.wa-modal__qr-frame {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 160px;
  height: 160px;
  border-radius: 0.75rem;
  border: 2px dashed rgb(var(--border));
  color: rgb(var(--muted));
}

.wa-modal__qr-frame.is-busy {
  border-style: solid;
  animation: wa-pulse 1.2s ease-in-out infinite;
}

.wa-modal__qr-frame-text {
  font-size: 0.85rem;
  letter-spacing: 0.15em;
  text-transform: uppercase;
}

.wa-modal__qr-ok {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: 999px;
  background: rgb(var(--success));
  color: #fff;
  font-size: 2rem;
  font-weight: 700;
}

.wa-modal__qr-hint {
  margin: 0;
  font-size: 0.85rem;
  color: rgb(var(--muted));
  max-width: 28rem;
}

.wa-modal__qr-sub {
  margin: 0;
  font-size: 0.8rem;
  color: rgb(var(--muted));
}

.wa-modal__footer {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.wa-modal__footer-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

@keyframes wa-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.45; }
}

@media (max-width: 640px) {
  .wa-modal__footer,
  .wa-modal__footer-actions {
    width: 100%;
  }

  .wa-modal__footer-actions {
    justify-content: flex-end;
  }
}
</style>
