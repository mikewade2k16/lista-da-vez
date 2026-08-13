<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import AdminAuthShell from '~/components/layout/AdminAuthShell.vue'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../../layers/core/stores/account'

definePageMeta({
  layout: 'auth',
})

useHead({
  title: 'Entrar | Omni',
})

const route = useRoute()
const auth = useAuthStore()
const coreAccount = useCoreAccountStore()

const form = reactive({
  email: '',
  password: '',
})
const rememberLogin = ref(false)
const showPassword = ref(false)
// submitting cobre o ciclo INTEIRO do submit, incluindo o navigateTo. auth.pending
// zera no finally de auth.login() ANTES da navegacao, entao sem isto o botao
// voltava a "Entrar" enquanto o usuario ainda estava na tela de login (parecia
// travado). Com o defer do runtime no store, a navegacao comeca logo apos o
// /v1/me/context e o loading so solta quando a rota destino pinta.
const submitting = ref(false)
const isBusy = computed(() => auth.pending || submitting.value)

onMounted(() => {
  const rememberedLogin = auth.getRememberedLogin()
  auth.lastError = ''

  if (!rememberedLogin) {
    return
  }

  form.email = rememberedLogin.email
  form.password = rememberedLogin.password
  rememberLogin.value = true
})

watch(rememberLogin, (enabled) => {
  if (!enabled) {
    auth.clearRememberedLogin()
  }
})

async function submitLogin() {
  submitting.value = true
  try {
    await auth.login({
      email: form.email,
      password: form.password,
    })

    if (rememberLogin.value) {
      auth.saveRememberedLogin({
        email: form.email,
        password: form.password,
      })
    } else {
      auth.clearRememberedLogin()
    }

    if (auth.mustChangePassword) {
      await navigateTo('/perfil', { replace: true })
      return
    }

    // Etapa 2 (authn != authz): se o login veio SEM papel-coarse (usuario so-agencia
    // ou so-papel-custom), a home nao pode sair do gating coarse legado (cairia em
    // 'operacao' vazio). Carrega o contexto de contas custom (v2) ANTES de decidir o
    // destino, para auth.homePath ja refletir os workspaces que a conta ativa concede.
    // fetchAccounts e idempotente/deduped; o auth.global re-resolve no destino de toda
    // forma (este await so evita o flash de roteamento para o caso novo).
    if (!auth.hasCoarseRole) {
      try {
        await coreAccount.fetchAccounts()
      } catch {
        // Falha transitoria nao deve travar o login; o auth.global re-resolve no destino.
      }
    }

    const redirectTarget = String(route.query.redirect || '').trim()
    const destination =
      redirectTarget && redirectTarget.startsWith('/') ? redirectTarget : auth.homePath
    await navigateTo(destination, { replace: true })
  } catch {
    // Erro de auth ja exibido via auth.lastError; o finally libera o form.
  } finally {
    // So solta o loading apos a navegacao (ou apos o erro) — mantem "Entrando..."
    // ate a rota destino pintar, sem botao "morto" na tela de login.
    submitting.value = false
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
          :readonly="isBusy"
          required
        />
      </div>

      <div class="admin-auth-field admin-auth-field--password">
        <input
          v-model="form.password"
          class="admin-auth-input"
          name="password"
          :type="showPassword ? 'text' : 'password'"
          autocomplete="current-password"
          placeholder="Senha"
          :readonly="isBusy"
          required
        />
        <button
          type="button"
          class="admin-auth-eye-btn"
          :aria-label="showPassword ? 'Ocultar senha' : 'Mostrar senha'"
          @click="showPassword = !showPassword"
        >
          <svg
            v-if="!showPassword"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.75"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
            <circle cx="12" cy="12" r="3" />
          </svg>
          <svg
            v-else
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.75"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path
              d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"
            />
            <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
            <line x1="1" y1="1" x2="23" y2="23" />
          </svg>
        </button>
      </div>

      <div class="admin-auth-options">
        <label class="admin-auth-checkbox">
          <input
            v-model="rememberLogin"
            type="checkbox"
            class="admin-auth-checkbox__check"
            autocomplete="off"
          />
          <span>Lembrar login</span>
        </label>
        <NuxtLink class="admin-auth-action" to="/auth/esqueceu-senha">Esqueceu a senha?</NuxtLink>
      </div>

      <Transition name="admin-auth-fade">
        <div v-if="auth.lastError" class="admin-auth-alert admin-auth-alert--error">
          {{ auth.lastError }}
        </div>
      </Transition>

      <button type="submit" class="admin-auth-submit" :disabled="isBusy">
        <span v-if="isBusy" class="admin-auth-submit__spinner"></span>
        <span>{{ isBusy ? 'Entrando...' : 'Entrar' }}</span>
      </button>

      <p class="admin-auth-meta">
        Se o acesso estiver inativo ou bloqueado, fale com um administrador.
      </p>
    </form>
  </AdminAuthShell>
</template>
