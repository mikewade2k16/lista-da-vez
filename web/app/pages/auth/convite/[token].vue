<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import AdminAuthShell from "~/components/layout/AdminAuthShell.vue";
import { getRoleLabel } from "~/domain/utils/permissions";
import { getApiErrorMessage } from "~/utils/api-client";
import { useAuthStore } from "~/stores/auth";

definePageMeta({
  layout: "auth"
});

useHead({
  title: "Aceitar convite | Fila de Atendimento"
});

const route = useRoute();
const auth = useAuthStore();

const invitation = ref<any | null>(null);
const pageError = ref("");
const formError = ref("");
const form = reactive({
  password: "",
  confirmPassword: ""
});
const showPassword = ref(false);
const showConfirmPassword = ref(false);

const token = computed(() => String(route.params.token || "").trim());
const shellTitle = computed(() => (invitation.value ? "Aceitar convite" : "Convite indisponivel"));
const shellDescription = computed(() => {
  if (invitation.value) {
    return "Defina sua senha para ativar o acesso inicial e entrar na plataforma.";
  }

  return pageError.value || "O link de convite esta indisponivel ou expirou.";
});

async function loadInvitation() {
  if (!token.value) {
    pageError.value = "Convite invalido.";
    return;
  }

  try {
    const response = await auth.fetchInvitation(token.value);
    invitation.value = response?.invitation || null;
    pageError.value = "";
  } catch (error) {
    invitation.value = null;
    pageError.value = getApiErrorMessage(error, "Nao foi possivel carregar o convite.");
  }
}

async function submitInvitation() {
  formError.value = "";

  if (String(form.password || "").trim().length < 6) {
    formError.value = "Defina uma senha com pelo menos 6 caracteres.";
    return;
  }

  if (form.password !== form.confirmPassword) {
    formError.value = "A confirmacao da senha nao confere.";
    return;
  }

  try {
    await auth.acceptInvitation({
      token: token.value,
      password: form.password
    });

    await navigateTo(auth.homePath, { replace: true });
  } catch {}
}

await loadInvitation();
</script>

<template>
  <AdminAuthShell :title="shellTitle" :description="shellDescription" card-width="30rem">
    <template v-if="invitation">
      <div class="admin-auth-code-grid admin-auth-summary-grid">
        <div class="admin-auth-summary-card">
          <span class="admin-auth-summary-label">Usuario</span>
          <strong class="admin-auth-summary-value">{{ invitation.displayName }}</strong>
        </div>
        <div class="admin-auth-summary-card">
          <span class="admin-auth-summary-label">Email</span>
          <strong class="admin-auth-summary-value">{{ invitation.email }}</strong>
        </div>
        <div class="admin-auth-summary-card">
          <span class="admin-auth-summary-label">Papel</span>
          <strong class="admin-auth-summary-value">{{ getRoleLabel(invitation.role) }}</strong>
        </div>
      </div>

      <form class="admin-auth-form" autocomplete="on" novalidate @submit.prevent="submitInvitation">
        <div class="admin-auth-field admin-auth-field--password">
          <input
            v-model="form.password"
            class="admin-auth-input"
            :type="showPassword ? 'text' : 'password'"
            autocomplete="new-password"
            placeholder="Nova senha"
            :readonly="auth.pending"
            required
          >
          <button
            type="button"
            class="admin-auth-eye-btn"
            :aria-label="showPassword ? 'Ocultar senha' : 'Mostrar senha'"
            @click="showPassword = !showPassword"
          >
            <svg v-if="!showPassword" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
              <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
              <line x1="1" y1="1" x2="23" y2="23" />
            </svg>
          </button>
        </div>

        <div class="admin-auth-field admin-auth-field--password">
          <input
            v-model="form.confirmPassword"
            class="admin-auth-input"
            :type="showConfirmPassword ? 'text' : 'password'"
            autocomplete="new-password"
            placeholder="Confirmar senha"
            :readonly="auth.pending"
            required
          >
          <button
            type="button"
            class="admin-auth-eye-btn"
            :aria-label="showConfirmPassword ? 'Ocultar senha' : 'Mostrar senha'"
            @click="showConfirmPassword = !showConfirmPassword"
          >
            <svg v-if="!showConfirmPassword" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
              <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
              <line x1="1" y1="1" x2="23" y2="23" />
            </svg>
          </button>
        </div>

        <Transition name="admin-auth-fade">
          <div v-if="formError || auth.lastError" class="admin-auth-alert admin-auth-alert--error">
            {{ formError || auth.lastError }}
          </div>
        </Transition>

        <button class="admin-auth-submit" type="submit" :disabled="auth.pending">
          <span v-if="auth.pending" class="admin-auth-submit__spinner" />
          <span>{{ auth.pending ? "Ativando..." : "Ativar acesso" }}</span>
        </button>

        <p class="admin-auth-meta">Use uma senha com pelo menos 6 caracteres para concluir o primeiro acesso.</p>
      </form>
    </template>

    <template v-else>
      <div class="admin-auth-alert admin-auth-alert--error">
        {{ pageError || "Convite indisponivel." }}
      </div>
      <NuxtLink class="admin-auth-submit" to="/auth/login">
        Voltar para login
      </NuxtLink>
    </template>
  </AdminAuthShell>
</template>
