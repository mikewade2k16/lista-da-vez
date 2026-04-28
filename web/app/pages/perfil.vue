<script setup>
import { computed, onMounted, reactive, ref, watch } from "vue";

import AppPanelButton from "~/components/ui/AppPanelButton.vue";
import { useAuthStore } from "~/stores/auth";
import { useUiStore } from "~/stores/ui";
import { getApiBase, getApiErrorMessage } from "~/utils/api-client";

definePageMeta({
  layout: "dashboard",
  workspaceId: "",
  pageLabel: "Perfil"
});

const runtimeConfig = useRuntimeConfig();
const auth = useAuthStore();
const ui = useUiStore();

onMounted(() => {
  void auth.ensureSession();
});

const profileDraft = reactive({
  displayName: "",
  email: ""
});
const passwordDraft = reactive({
  currentPassword: "",
  newPassword: "",
  confirmPassword: ""
});
const avatarPending = ref(false);
const profilePending = ref(false);
const passwordPending = ref(false);

const avatarUrl = computed(() => {
  const avatarPath = String(auth.user?.avatarPath || "").trim();
  if (!avatarPath) {
    return "";
  }

  return new URL(avatarPath, getApiBase(runtimeConfig)).toString();
});

const initials = computed(() =>
  String(auth.user?.displayName || "")
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((chunk) => chunk[0] || "")
    .join("")
    .toUpperCase()
);

watch(
  () => auth.user,
  (user) => {
    profileDraft.displayName = String(user?.displayName || "");
    profileDraft.email = String(user?.email || "");
  },
  {
    immediate: true,
    deep: true
  }
);

async function saveProfile() {
  profilePending.value = true;

  try {
    await auth.updateProfile(profileDraft);
    ui.success("Perfil atualizado.");
  } catch (error) {
    ui.error(getApiErrorMessage(error, "Nao foi possivel atualizar o perfil."));
  } finally {
    profilePending.value = false;
  }
}

async function changePassword() {
  if (String(passwordDraft.newPassword || "").trim().length < 8) {
    ui.error("A nova senha deve ter pelo menos 8 caracteres.");
    return;
  }

  if (passwordDraft.newPassword !== passwordDraft.confirmPassword) {
    ui.error("A confirmacao da senha nao confere.");
    return;
  }

  passwordPending.value = true;

  try {
    await auth.changePassword(passwordDraft);
    passwordDraft.currentPassword = "";
    passwordDraft.newPassword = "";
    passwordDraft.confirmPassword = "";
    ui.success("Senha alterada.");
  } catch (error) {
    ui.error(getApiErrorMessage(error, "Nao foi possivel alterar a senha."));
  } finally {
    passwordPending.value = false;
  }
}

async function handleAvatarChange(event) {
  const file = event?.target?.files?.[0] || null;
  if (!file) {
    return;
  }

  avatarPending.value = true;

  try {
    await auth.uploadAvatar(file);
    ui.success("Foto atualizada.");
  } catch (error) {
    ui.error(getApiErrorMessage(error, "Nao foi possivel enviar a foto."));
  } finally {
    avatarPending.value = false;
    event.target.value = "";
  }
}
</script>

<template>
  <div class="page-workspace">
  <section class="admin-panel profile-panel" data-testid="profile-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Meu perfil</h2>
      <p class="admin-panel__text">Atualize sua foto, nome, email e senha sem depender do administrativo.</p>
    </header>

    <article v-if="auth.mustChangePassword" class="insight-card">
      <p class="settings-card__text">
        Sua conta ainda está com senha temporária. Antes de continuar usando a plataforma, atualize sua senha abaixo.
      </p>
    </article>

    <div class="profile-panel__grid">
      <article class="settings-card profile-panel__avatar-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Foto</h3>
          <p class="settings-card__text">JPG, PNG ou WebP com ate 2 MB.</p>
        </header>

        <div class="profile-panel__avatar-wrap">
          <img v-if="avatarUrl" :src="avatarUrl" alt="Foto do usuario" class="profile-panel__avatar-image">
          <span v-else class="profile-panel__avatar-fallback">{{ initials || "US" }}</span>
        </div>

        <AppPanelButton as="label" block class="profile-panel__avatar-button" :disabled="avatarPending">
          <input type="file" accept="image/png,image/jpeg,image/webp" hidden @change="handleAvatarChange">
          {{ avatarPending ? "Enviando..." : "Enviar nova foto" }}
        </AppPanelButton>
      </article>

      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Dados pessoais</h3>
          <p class="settings-card__text">Esses dados aparecem na conta e nas areas autenticadas.</p>
        </header>

        <form class="multistore-form multistore-form--add" @submit.prevent="saveProfile">
          <div class="multistore-form__row">
            <input v-model="profileDraft.displayName" class="product-add__input" type="text" placeholder="Nome completo *">
            <input v-model="profileDraft.email" class="product-add__input" type="email" placeholder="Email *">
          </div>
          <div class="multistore-form__actions">
            <AppPanelButton type="submit" :disabled="profilePending">
              {{ profilePending ? "Salvando..." : "Salvar perfil" }}
            </AppPanelButton>
          </div>
        </form>
      </article>
    </div>

    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Senha</h3>
        <p class="settings-card__text">
          {{ auth.mustChangePassword ? "Defina agora sua senha pessoal para liberar o restante do sistema." : "Troque sua senha mantendo a conta protegida." }}
        </p>
      </header>

      <form class="multistore-form multistore-form--add" @submit.prevent="changePassword">
        <div class="multistore-form__row">
          <input v-model="passwordDraft.currentPassword" class="product-add__input" type="password" :placeholder="auth.mustChangePassword ? 'Senha temporaria atual *' : 'Senha atual *'">
          <input v-model="passwordDraft.newPassword" class="product-add__input" type="password" placeholder="Nova senha *">
          <input v-model="passwordDraft.confirmPassword" class="product-add__input" type="password" placeholder="Confirmar nova senha *">
        </div>
        <div class="multistore-form__actions">
          <AppPanelButton type="submit" :disabled="passwordPending">
            {{ passwordPending ? "Atualizando..." : auth.mustChangePassword ? "Definir minha senha" : "Atualizar senha" }}
          </AppPanelButton>
        </div>
      </form>
    </article>
  </section>
  </div>
</template>

<style scoped>
.profile-panel__avatar-button {
  margin-top: auto;
}
</style>
