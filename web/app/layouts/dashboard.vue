<script setup>
import { computed, onMounted, ref } from 'vue'
import DashboardHeader from '~/components/dashboard/DashboardHeader.vue'
import DashboardWorkspaceNav from '~/components/dashboard/DashboardWorkspaceNav.vue'
import FeedbackFormModal from '~/components/feedback/FeedbackFormModal.vue'
import { useContextRealtime } from '~/composables/useContextRealtime'
import { useDashboardShell } from '~/composables/useDashboardShell'
import { useAuthStore } from '~/stores/auth'
import { useMenuLayoutStore } from '~/stores/menuLayout'
import { useCoreAccountStore } from '../../layers/core/stores/account'

const { state, activeWorkspaceId, allowedWorkspaces, setActiveProfile } = useDashboardShell()
const auth = useAuthStore()
const accountStore = useCoreAccountStore()
const route = useRoute()
const menuLayout = useMenuLayoutStore()
useContextRealtime()

// Carrega a config global do menu (header vs sidebar) uma unica vez no client.
// Idempotente: o proprio store ignora chamadas repetidas e nunca bloqueia o
// shell (sem layout salvo = default 'both', comportamento de hoje).
onMounted(() => {
  void menuLayout.load()
})

const feedbackModalOpen = ref(false)
const runtimeSettingsNotice = computed(() => String(auth.runtimeSettingsNotice || '').trim())
// Conta sem o modulo queue nao usa configuracoes de fila — "configuracoes
// indisponiveis" e o estado esperado, nao um erro. O aviso de modo degradado so
// faz sentido para quem TEM queue (e portanto deveria conseguir carregar /v1/settings).
const hasQueueModule = computed(() => accountStore.enabledModules.includes('queue'))
const usesQueueWorkspace = computed(() => String(route.path || '').startsWith('/operacao'))

function handleProfileChange(profileId) {
  void setActiveProfile(profileId)
}
</script>

<template>
  <main class="shell">
    <section class="app-surface">
      <ClientOnly>
        <DashboardHeader
          :state="state"
          :show-operations-context="false"
          :active-workspace="activeWorkspaceId"
          :allowed-workspaces="allowedWorkspaces"
          @profile-change="handleProfileChange"
        />
      </ClientOnly>
      <div
        v-if="runtimeSettingsNotice && hasQueueModule"
        class="runtime-settings-banner"
        role="status"
        aria-live="polite"
      >
        <span class="material-icons-round runtime-settings-banner__icon" aria-hidden="true">
          warning
        </span>
        <div class="runtime-settings-banner__body">
          <strong>Modo degradado de configuracoes</strong>
          <p>{{ runtimeSettingsNotice }}</p>
        </div>
      </div>
      <div v-if="usesQueueWorkspace" class="workspace">
        <DashboardWorkspaceNav
          :active-workspace="activeWorkspaceId"
          :allowed-workspaces="allowedWorkspaces"
          :state="state"
          show-operations-context
          @profile-change="handleProfileChange"
        />
        <slot></slot>
      </div>
      <div v-else class="module-workspace-full">
        <slot></slot>
      </div>
    </section>

    <button
      class="dashboard-feedback-btn"
      title="Enviar feedback para o time"
      aria-label="Enviar feedback"
      @click="feedbackModalOpen = true"
    >
      <span class="dashboard-feedback-btn__icon">💬</span>
    </button>

    <FeedbackFormModal v-model="feedbackModalOpen" />
  </main>
</template>

<style scoped>
.runtime-settings-banner {
  display: flex;
  align-items: flex-start;
  gap: 0.85rem;
  margin: 1rem 1.2rem 0;
  padding: 0.95rem 1rem;
  border-radius: 18px;
  border: 1px solid rgba(245, 158, 11, 0.35);
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.16), rgba(15, 23, 42, 0.92));
  color: #fef3c7;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.24);
}

.runtime-settings-banner__icon {
  font-size: 1.15rem;
  line-height: 1;
  margin-top: 0.1rem;
  color: #fbbf24;
}

.runtime-settings-banner__body {
  display: grid;
  gap: 0.2rem;
}

.runtime-settings-banner__body strong {
  font-size: 0.92rem;
  letter-spacing: 0.01em;
}

.runtime-settings-banner__body p {
  margin: 0;
  color: rgba(255, 247, 237, 0.88);
  font-size: 0.88rem;
  line-height: 1.45;
}

.workspace {
  position: relative;
  z-index: 0;
}

.module-workspace-full {
  position: relative;
  z-index: 0;
  width: 100%;
  max-width: none;
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 8px 12px 0;
}

/* A aurora ambiente (liquid glass) agora vive no TEMA `.theme-liquidglass`
   (assets/styles/omni-tokens.css), gateada pelo Theme Studio — não mais
   sempre-ligada aqui. Os containers de página (.module-workspace-full base e
   .workspace) só mantêm position/z-index/overflow para conter a camada quando o
   tema está ativo. */
.dashboard-feedback-btn {
  position: fixed;
  bottom: 2rem;
  right: 2rem;
  width: 3rem;
  height: 3rem;
  border-radius: 50%;
  /* tokens do design system (sem hex hardcoded) — acompanha o tema. */
  background-color: rgb(var(--primary));
  color: rgb(255 255 255);
  border: none;
  cursor: pointer;
  box-shadow: 0 4px 12px rgb(var(--primary) / 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  transition:
    background-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
  z-index: 500;
}

.dashboard-feedback-btn:hover {
  background-color: rgb(var(--primary-600));
  box-shadow: 0 6px 16px rgb(var(--primary) / 0.4);
  transform: scale(1.1);
}

.dashboard-feedback-btn:active {
  transform: scale(0.95);
}

/* Botao flutuante em vidro — SO no tema Liquid Glass. */
.theme-liquidglass .dashboard-feedback-btn {
  border: 1px solid var(--glass-border);
  background: var(--glass-highlight), rgb(var(--primary) / 0.55);
  backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
  -webkit-backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
  box-shadow:
    0 10px 28px rgb(var(--primary) / 0.35),
    inset 0 1px 0 rgb(255 255 255 / 0.25);
}

.dashboard-feedback-btn__icon {
  font-size: 1.5rem;
  line-height: 1;
}
</style>
