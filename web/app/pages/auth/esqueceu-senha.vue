<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import AdminAuthShell from "~/components/layout/AdminAuthShell.vue";
import { useAuthStore } from "~/stores/auth";

definePageMeta({
  layout: "auth"
});

useHead({
  title: "Recuperar senha | Fila de Atendimento"
});

type RecoveryStep = "request" | "confirm" | "success";

const auth = useAuthStore();
const step = ref<RecoveryStep>("request");
const requestError = ref("");
const confirmError = ref("");
const showPassword = ref(false);
const showConfirmPassword = ref(false);
const requestedEmail = ref("");

const requestForm = reactive({
  email: ""
});

const confirmForm = reactive({
  code: "",
  password: "",
  confirmPassword: ""
});

const shellTitle = computed(() => {
  switch (step.value) {
    case "confirm":
      return "Confirmar codigo";
    case "success":
      return "Senha redefinida";
    default:
      return "Recuperar senha";
  }
});

const shellDescription = computed(() => {
  switch (step.value) {
    case "confirm":
      return "Digite o codigo recebido e defina sua nova senha para voltar a entrar no sistema.";
    case "success":
      return "Sua senha foi atualizada. Use a nova senha no proximo login.";
    default:
      return "Informe o email do seu acesso. Vamos enviar um codigo para recuperar a senha.";
  }
});

const maskedRequestedEmail = computed(() => maskEmail(requestedEmail.value || requestForm.email));

onMounted(() => {
  const rememberedLogin = auth.getRememberedLogin();
  auth.lastError = "";

  if (rememberedLogin?.email) {
    requestForm.email = rememberedLogin.email;
  }
});

function normalizeEmail(value: string) {
  return String(value || "").trim().toLowerCase();
}

function sanitizeCode(value: string) {
  return String(value || "").replace(/\D+/g, "").slice(0, 6);
}

function maskEmail(value: string) {
  const normalized = normalizeEmail(value);
  const [localPart, domain] = normalized.split("@");

  if (!localPart || !domain) {
    return normalized;
  }

  const visibleStart = localPart.slice(0, 2);
  const hiddenLength = Math.max(localPart.length - visibleStart.length, 1);
  return `${visibleStart}${"*".repeat(hiddenLength)}@${domain}`;
}

function resetConfirmForm() {
  confirmForm.code = "";
  confirmForm.password = "";
  confirmForm.confirmPassword = "";
  confirmError.value = "";
  auth.lastError = "";
}

async function submitRequest() {
  requestError.value = "";
  confirmError.value = "";
  auth.lastError = "";

  const email = normalizeEmail(requestForm.email);
  if (!email) {
    requestError.value = "Informe o email do seu acesso.";
    return;
  }

  try {
    await auth.requestPasswordReset({ email });
    requestedEmail.value = email;
    step.value = "confirm";
    resetConfirmForm();
  } catch {}
}

async function submitConfirm() {
  confirmError.value = "";
  auth.lastError = "";
  confirmForm.code = sanitizeCode(confirmForm.code);

  if (confirmForm.code.length !== 6) {
    confirmError.value = "Digite o codigo de 6 digitos enviado para o seu email.";
    return;
  }

  if (String(confirmForm.password || "").trim().length < 8) {
    confirmError.value = "Defina uma senha com pelo menos 8 caracteres.";
    return;
  }

  if (confirmForm.password !== confirmForm.confirmPassword) {
    confirmError.value = "A confirmacao da senha nao confere.";
    return;
  }

  try {
    await auth.confirmPasswordReset({
      email: requestedEmail.value,
      code: confirmForm.code,
      password: confirmForm.password
    });
    auth.clearRememberedLogin();
    step.value = "success";
  } catch {}
}

function goBackToRequest() {
  step.value = "request";
  confirmError.value = "";
  auth.lastError = "";
}
</script>

<template>
  <AdminAuthShell :title="shellTitle" :description="shellDescription" card-width="30rem">
    <template v-if="step === 'request'">
      <form class="admin-auth-form" autocomplete="on" novalidate @submit.prevent="submitRequest">
        <div class="admin-auth-field">
          <input
            v-model="requestForm.email"
            class="admin-auth-input"
            type="email"
            autocomplete="username"
            inputmode="email"
            autocapitalize="none"
            placeholder="Email"
            :readonly="auth.pending"
            required
          >
        </div>

        <Transition name="admin-auth-fade">
          <div v-if="requestError || auth.lastError" class="admin-auth-alert admin-auth-alert--error">
            {{ requestError || auth.lastError }}
          </div>
        </Transition>

        <button class="admin-auth-submit" type="submit" :disabled="auth.pending">
          <span v-if="auth.pending" class="admin-auth-submit__spinner" />
          <span>{{ auth.pending ? "Enviando..." : "Enviar codigo" }}</span>
        </button>

        <div class="admin-auth-actions">
          <NuxtLink class="admin-auth-link" to="/auth/login">
            Voltar para login
          </NuxtLink>
        </div>
      </form>
    </template>

    <template v-else-if="step === 'confirm'">
      <div class="admin-auth-code-grid">
        <div class="admin-auth-summary-card">
          <span class="admin-auth-summary-label">Codigo enviado para</span>
          <strong class="admin-auth-summary-value">{{ maskedRequestedEmail }}</strong>
        </div>
      </div>

      <form class="admin-auth-form" autocomplete="off" novalidate @submit.prevent="submitConfirm">
        <div class="admin-auth-field">
          <input
            v-model="confirmForm.code"
            class="admin-auth-input admin-auth-input--code"
            type="text"
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength="6"
            placeholder="000000"
            :readonly="auth.pending"
            @input="confirmForm.code = sanitizeCode(confirmForm.code)"
          >
        </div>

        <div class="admin-auth-field admin-auth-field--password">
          <input
            v-model="confirmForm.password"
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
            v-model="confirmForm.confirmPassword"
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
          <div v-if="confirmError || auth.lastError" class="admin-auth-alert admin-auth-alert--error">
            {{ confirmError || auth.lastError }}
          </div>
        </Transition>

        <button class="admin-auth-submit" type="submit" :disabled="auth.pending">
          <span v-if="auth.pending" class="admin-auth-submit__spinner" />
          <span>{{ auth.pending ? "Redefinindo..." : "Redefinir senha" }}</span>
        </button>

        <div class="admin-auth-actions">
          <button type="button" class="admin-auth-action" :disabled="auth.pending" @click="goBackToRequest">
            Alterar email
          </button>
          <button type="button" class="admin-auth-action" :disabled="auth.pending" @click="submitRequest">
            Reenviar codigo
          </button>
        </div>
      </form>
    </template>

    <template v-else>
      <div class="admin-auth-alert admin-auth-alert--success">
        Sua senha foi atualizada com sucesso. Agora voce ja pode entrar usando a nova senha.
      </div>

      <NuxtLink class="admin-auth-submit" to="/auth/login">
        Ir para login
      </NuxtLink>
    </template>
  </AdminAuthShell>
</template>