<script setup lang="ts">
import { UAlert, UBadge, UButton, UFormField, UInput, UModal } from "#components";
import { computed, ref, watch } from "vue";
import type { Contact } from "~/types";

type SaveContactDraft = {
  name: string;
  phone: string;
  avatarUrl?: string | null;
  contactId?: string | null;
};

const props = defineProps<{
  open: boolean;
  payload: SaveContactDraft | null;
  existingContact: Contact | null;
  saving: boolean;
  errorMessage: string;
  formatDisplayPhone: (value: string | null | undefined) => string;
}>();

const emit = defineEmits<{
  (event: "close"): void;
  (event: "save", payload: SaveContactDraft): void;
  (event: "open-existing", contactId: string): void;
}>();

const name = ref("");
const phone = ref("");

watch(
  () => props.payload,
  (value) => {
    name.value = value?.name ?? "";
    phone.value = props.formatDisplayPhone(value?.phone);
  },
  { immediate: true }
);

const normalizedDisplayPhone = computed(() => props.formatDisplayPhone(phone.value));
const actionLabel = computed(() => (props.existingContact ? "Atualizar contato" : "Salvar contato"));
const helperMessage = computed(() => {
  if (!props.existingContact) {
    return "Esse contato ainda nao existe na sua agenda interna.";
  }

  return `Ja existe um contato salvo para este numero: ${props.existingContact.name}. Se salvar, o cadastro sera atualizado.`;
});

function handleClose() {
  emit("close");
}

function handleSave() {
  if (!props.payload) {
    return;
  }

  emit("save", {
    ...props.payload,
    name: name.value.trim() || normalizedDisplayPhone.value || phone.value.trim(),
    phone: phone.value.trim()
  });
}

function handleOpenExisting() {
  if (!props.existingContact?.id) {
    return;
  }

  emit("open-existing", props.existingContact.id);
}
</script>

<template>
  <UModal :open="open" title="Salvar contato" description="Valide os dados antes de salvar este contato." @update:open="!$event && handleClose()">
    <template #body>
      <div class="contact-save-modal">
        <UAlert
          :color="existingContact ? 'warning' : 'success'"
          variant="soft"
          :title="existingContact ? 'Contato ja existe' : 'Novo contato'"
          :description="helperMessage"
        >
          <template #actions>
            <UBadge v-if="existingContact" color="warning" variant="subtle">
              {{ existingContact.name }}
            </UBadge>
          </template>
        </UAlert>

        <UFormField label="Nome">
          <UInput v-model="name" placeholder="Nome do contato" />
        </UFormField>

        <UFormField label="Telefone">
          <UInput v-model="phone" placeholder="Telefone completo" />
          <template #help>
            <span class="contact-save-modal__help">
              Exibicao: {{ normalizedDisplayPhone || "numero invalido" }}
            </span>
          </template>
        </UFormField>

        <UAlert
          v-if="errorMessage"
          color="error"
          variant="soft"
          title="Nao foi possivel salvar"
          :description="errorMessage"
        />
      </div>
    </template>

    <template #footer>
      <div class="contact-save-modal__actions">
        <UButton size="sm" color="neutral" variant="ghost" @click="handleClose">
          Cancelar
        </UButton>
        <UButton
          v-if="existingContact?.id"
          size="sm"
          color="neutral"
          variant="outline"
          @click="handleOpenExisting"
        >
          Abrir conversa
        </UButton>
        <UButton size="sm" color="primary" :loading="saving" @click="handleSave">
          {{ actionLabel }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.contact-save-modal {
  display: grid;
  gap: 0.85rem;
}

.contact-save-modal__actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}

.contact-save-modal__help {
  color: rgb(var(--muted));
  font-size: 0.74rem;
}
</style>
