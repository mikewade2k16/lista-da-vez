<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'

// Bloco "Seguranca e sessao": quando a sessao expira (token de 12h SEM refresh,
// entao saber a expiracao e util), sair da conta e esquecer o login salvo neste
// navegador. Tudo com dado/acoes que ja existem (principal.expiresAt, auth.logout,
// auth.clearRememberedLogin). Nenhuma chamada nova.
const auth = useAuthStore()

const loggingOut = ref(false)
const hasRemembered = ref(false)

const expiresAt = computed(() => {
  const raw = auth.principal?.expiresAt
  if (!raw) {
    return null
  }
  const ms = Date.parse(String(raw))
  return Number.isFinite(ms) ? new Date(ms) : null
})

const expiresLabel = computed(() => {
  const date = expiresAt.value
  if (!date) {
    return 'Sem informacao de expiracao.'
  }
  const time = date.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })
  const day = date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' })
  return `${day} as ${time}`
})

const expiresRelative = computed(() => {
  const date = expiresAt.value
  if (!date) {
    return ''
  }
  const diffMs = date.getTime() - Date.now()
  if (diffMs <= 0) {
    return 'expirada'
  }
  const hours = Math.floor(diffMs / 3_600_000)
  const minutes = Math.floor((diffMs % 3_600_000) / 60_000)
  if (hours > 0) {
    return `faltam ~${hours}h${minutes > 0 ? ` ${minutes}min` : ''}`
  }
  return `faltam ~${minutes}min`
})

onMounted(() => {
  hasRemembered.value = Boolean(auth.getRememberedLogin())
})

async function handleLogout() {
  loggingOut.value = true
  try {
    await auth.logout()
    await navigateTo('/auth/login', { replace: true })
  } finally {
    loggingOut.value = false
  }
}

function forgetRememberedLogin() {
  auth.clearRememberedLogin()
  hasRemembered.value = false
}
</script>

<template>
  <article class="settings-card profile-security">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Seguranca e sessao</h3>
      <p class="settings-card__text">Sua sessao expira automaticamente — sem login eterno.</p>
    </header>

    <div class="profile-security__session">
      <span class="profile-security__label">Sua sessao expira</span>
      <span class="profile-security__value">
        {{ expiresLabel }}
        <span v-if="expiresRelative" class="profile-security__hint">({{ expiresRelative }})</span>
      </span>
    </div>

    <div v-if="hasRemembered" class="profile-security__row">
      <div class="profile-security__row-text">
        <strong>Login lembrado neste navegador</strong>
        <p>Seu email e senha ficam salvos localmente para entrar mais rapido.</p>
      </div>
      <AppPanelButton variant="ghost" @click="forgetRememberedLogin">Esquecer</AppPanelButton>
    </div>

    <div class="profile-security__actions">
      <AppPanelButton variant="danger" :disabled="loggingOut" @click="handleLogout">
        {{ loggingOut ? 'Saindo...' : 'Sair da conta' }}
      </AppPanelButton>
    </div>
  </article>
</template>

<style scoped>
.profile-security__session {
  display: grid;
  gap: 0.25rem;
}

.profile-security__label {
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.profile-security__value {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-main);
}

.profile-security__hint {
  font-weight: 400;
  color: var(--text-muted);
}

.profile-security__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  margin-top: 1rem;
  padding: 0.8rem 0.9rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: var(--bg-muted);
}

.profile-security__row-text strong {
  font-size: 0.85rem;
  color: var(--text-main);
}

.profile-security__row-text p {
  margin: 0.15rem 0 0;
  font-size: 0.78rem;
  color: var(--text-muted);
}

.profile-security__actions {
  display: flex;
  justify-content: flex-start;
  margin-top: 1rem;
}
</style>
