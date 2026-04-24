<script setup lang="ts">
import { onMounted, reactive, ref, watch } from "vue";
import AdminAuthShell from "~/components/layout/AdminAuthShell.vue";
import { useAuthStore } from "~/stores/auth";

definePageMeta({
  layout: "auth"
});

useHead({
  title: "Entrar | Fila de Atendimento"
});

const route = useRoute();
const auth = useAuthStore();

const form = reactive({
   email: "",
   password: ""
});
const rememberLogin = ref(false);
const showPassword = ref(false);

onMounted(() => {
  const rememberedLogin = auth.getRememberedLogin();
  auth.lastError = "";

  if (!rememberedLogin) {
    return;
  }

  form.email = rememberedLogin.email;
  form.password = rememberedLogin.password;
  rememberLogin.value = true;
});

watch(rememberLogin, (enabled) => {
  if (!enabled) {
    auth.clearRememberedLogin();
  }
});

async function submitLogin() {
  try {
    await auth.login({
      email: form.email,
      password: form.password
    });

    if (rememberLogin.value) {
      auth.saveRememberedLogin({
        email: form.email,
        password: form.password
      });
    } else {
      auth.clearRememberedLogin();
    }

    if (auth.mustChangePassword) {
      await navigateTo("/perfil", { replace: true });
      return;
    }

    const redirectTarget = String(route.query.redirect || "").trim();
    const destination = redirectTarget && redirectTarget.startsWith("/") ? redirectTarget : auth.homePath;
    await navigateTo(destination, { replace: true });
  } catch {
    return;
  }
}
</script>

<template>
  <AdminAuthShell title="" description="" card-width="26rem">
    <form class="admin-auth-form" autocomplete="on" novalidate @submit.prevent="submitLogin">
      <div class="admin-auth-field">
        <input
          v-model="form.email"
          class="admin-auth-input"
          name="username"
          type="email"
          autocomplete="username"
          inputmode="email"
          autocapitalize="none"
          placeholder="Email"
          :readonly="auth.pending"
          required
        >
      </div>

      <div class="admin-auth-field admin-auth-field--password">
        <input
          v-model="form.password"
          class="admin-auth-input"
          name="password"
          :type="showPassword ? 'text' : 'password'"
          autocomplete="current-password"
          placeholder="Senha"
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

      <div class="admin-auth-options">
        <label class="admin-auth-checkbox">
          <input v-model="rememberLogin" type="checkbox" class="admin-auth-checkbox__check" autocomplete="off">
          <span>Lembrar login</span>
        </label>
        <NuxtLink class="admin-auth-action" to="/auth/esqueceu-senha">
          Esqueceu a senha?
        </NuxtLink>
      </div>

      <Transition name="admin-auth-fade">
        <div v-if="auth.lastError" class="admin-auth-alert admin-auth-alert--error">
          {{ auth.lastError }}
        </div>
      </Transition>

      <button type="submit" class="admin-auth-submit" :disabled="auth.pending">
        <span v-if="auth.pending" class="admin-auth-submit__spinner" />
        <span>{{ auth.pending ? "Entrando..." : "Entrar" }}</span>
      </button>

      <p class="admin-auth-meta">
        Se o acesso estiver inativo, bloqueado ou sem cliente vinculado, fale com um administrador.
      </p>
    </form>
  </AdminAuthShell>
</template>
