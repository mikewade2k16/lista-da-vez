<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useMetaAdsConnectionContext } from '~/composables/useMetaAdsConnectionContext'
import { useMetaAdsStore } from '~/stores/meta-ads'
import { useCoreAccountStore } from '../../../layers/core/stores/account'

const store = useMetaAdsStore()
const accountStore = useCoreAccountStore()
const connectionContext = useMetaAdsConnectionContext(accountStore.activeAccountId)

const { token, oauthPending, oauthError } = connectionContext

watch(
  () => accountStore.activeAccountId,
  (accountId) => connectionContext.bindAccount(accountId),
  { flush: 'sync' },
)

const canSubmitConnection = computed(
  () =>
    store.canConnectMetaAds &&
    token.value.trim().length > 0 &&
    !store.connecting &&
    !oauthPending.value,
)

async function onConnect() {
  if (!canSubmitConnection.value) return
  const snapshot = connectionContext.capture()
  const submittedToken = token.value.trim()
  if (!connectionContext.isCurrent(snapshot, accountStore.activeAccountId)) return

  await store.saveConnection(submittedToken)
  if (!connectionContext.isCurrent(snapshot, accountStore.activeAccountId)) return
  // Nunca ecoar o token de volta: limpa o campo apos a tentativa.
  token.value = ''
}

async function onDisconnect() {
  if (!store.canConnectMetaAds) return
  await store.deleteConnection()
}

function scheduleOAuthPoll(
  snapshot: ReturnType<typeof connectionContext.capture>,
  previousRevision: string,
) {
  connectionContext.schedulePoll(
    snapshot,
    () => void pollOAuthConnection(snapshot, previousRevision),
  )
}

async function pollOAuthConnection(
  snapshot: ReturnType<typeof connectionContext.capture>,
  previousRevision: string,
) {
  if (!oauthPending.value || !connectionContext.isCurrent(snapshot, accountStore.activeAccountId))
    return
  const attempt = connectionContext.nextPollAttempt(snapshot)
  if (attempt === null) return
  try {
    await store.loadOverview()
  } catch {
    // Falha transitoria de rede nao invalida o state nem encerra o popup.
  }
  if (!connectionContext.isCurrent(snapshot, accountStore.activeAccountId)) return
  if (store.connected && (!previousRevision || store.connection?.revision !== previousRevision)) {
    await store.init()
    if (!connectionContext.isCurrent(snapshot, accountStore.activeAccountId)) return
    connectionContext.stopIfCurrent(snapshot, true)
    return
  }
  if (connectionContext.getPopup()?.closed || attempt >= 400) {
    connectionContext.stopIfCurrent(snapshot)
    return
  }
  scheduleOAuthPoll(snapshot, previousRevision)
}

async function onOAuthConnect() {
  if (!store.canConnectMetaAds || store.connecting || oauthPending.value || !import.meta.client)
    return
  const snapshot = connectionContext.capture()
  const previousRevision = store.connection?.revision || ''
  if (!connectionContext.isCurrent(snapshot, accountStore.activeAccountId)) return
  connectionContext.setError(snapshot, '')
  const popup = window.open('about:blank', 'omni-meta-oauth', 'popup,width=620,height=760')
  if (!popup) {
    connectionContext.setError(
      snapshot,
      'O navegador bloqueou a janela de login. Libere pop-ups e tente novamente.',
    )
    return
  }
  if (!connectionContext.setPopup(snapshot, popup)) return

  const result = await store.startConnectionOAuth()
  if (!connectionContext.isCurrent(snapshot, accountStore.activeAccountId)) {
    popup.close()
    return
  }
  if (!result) {
    connectionContext.stopIfCurrent(snapshot, true)
    connectionContext.setError(snapshot, store.error)
    return
  }
  try {
    const authorizationURL = new URL(result.authorizationUrl)
    const trustedHost =
      authorizationURL.hostname === 'facebook.com' ||
      authorizationURL.hostname.endsWith('.facebook.com')
    if (authorizationURL.protocol !== 'https:' || !trustedHost) throw new Error('invalid_oauth_url')
    popup.location.replace(authorizationURL.toString())
  } catch {
    connectionContext.stopIfCurrent(snapshot, true)
    connectionContext.setError(snapshot, 'O servidor devolveu uma URL de autorizacao invalida.')
    return
  }
  if (!connectionContext.setPending(snapshot, true)) {
    popup.close()
    return
  }
  scheduleOAuthPoll(snapshot, previousRevision)
}

onBeforeUnmount(() => connectionContext.dispose())

const expiresLabel = computed(() => {
  const raw = store.connection?.tokenExpiresAt
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleDateString('pt-BR')
})

const expiryState = computed<'ok' | 'warning' | 'expired' | ''>(() => {
  const raw = store.connection?.tokenExpiresAt
  if (!raw) return ''
  const expiresAt = new Date(raw).getTime()
  if (!Number.isFinite(expiresAt)) return ''
  const remainingDays = (expiresAt - Date.now()) / 86_400_000
  if (remainingDays <= 0) return 'expired'
  if (remainingDays <= 14) return 'warning'
  return 'ok'
})
</script>

<template>
  <article class="ma-connection">
    <header class="ma-connection__head">
      <div class="ma-connection__head-text">
        <h2 class="ma-connection__title">Conexao com a Meta</h2>
        <p class="ma-connection__subtitle">
          Autorize pelo Facebook Login para puxar contas, campanhas e metricas sem copiar tokens.
        </p>
      </div>
      <span
        class="ma-connection__status"
        :class="store.connected ? 'ma-connection__status--on' : 'ma-connection__status--off'"
      >
        <span class="ma-connection__dot" aria-hidden="true"></span>
        {{ store.connected ? 'Conectado' : 'Desconectado' }}
      </span>
    </header>

    <hr class="ma-connection__divider" />

    <div v-if="store.connected" class="ma-connection__connected">
      <dl class="ma-connection__facts">
        <div class="ma-connection__fact">
          <dt class="ma-connection__fact-label">Negocio</dt>
          <dd class="ma-connection__fact-value">{{ store.connection?.name || '—' }}</dd>
        </div>
        <div class="ma-connection__fact">
          <dt class="ma-connection__fact-label">Business ID</dt>
          <dd class="ma-connection__fact-value">{{ store.connection?.metaBusinessId || '—' }}</dd>
        </div>
        <div v-if="expiresLabel" class="ma-connection__fact">
          <dt class="ma-connection__fact-label">Token expira em</dt>
          <dd class="ma-connection__fact-value">{{ expiresLabel }}</dd>
        </div>
      </dl>
      <p
        v-if="expiryState === 'warning'"
        class="ma-connection__expiry ma-connection__expiry--warning"
      >
        O acesso está próximo de expirar. Renove agora para não interromper o sync.
      </p>
      <p
        v-else-if="expiryState === 'expired'"
        class="ma-connection__expiry ma-connection__expiry--danger"
      >
        O token expirou. Renove o acesso antes de sincronizar ou executar ações.
      </p>
      <div class="ma-connection__connected-actions">
        <button
          type="button"
          class="ma-connection__btn ma-connection__btn--primary"
          :disabled="store.connecting || oauthPending || !store.canConnectMetaAds"
          @click="onOAuthConnect"
        >
          {{ oauthPending ? 'Aguardando autorização…' : 'Renovar acesso' }}
        </button>
        <button
          type="button"
          class="ma-connection__btn ma-connection__btn--ghost"
          :disabled="!store.canConnectMetaAds"
          :title="
            store.canConnectMetaAds
              ? 'Remover a conexão com a Meta'
              : 'Sua função não pode alterar conexões do Meta Ads'
          "
          @click="onDisconnect"
        >
          Desconectar
        </button>
      </div>
    </div>

    <div v-else class="ma-connection__form">
      <button
        type="button"
        class="ma-connection__btn ma-connection__btn--primary"
        :disabled="store.connecting || oauthPending || !store.canConnectMetaAds"
        title="Conectar com o Facebook Login"
        @click="onOAuthConnect"
      >
        <span
          v-if="store.connecting || oauthPending"
          class="ma-connection__spinner"
          aria-hidden="true"
        ></span>
        {{ oauthPending ? 'Aguardando autorizacao...' : 'Conectar com Facebook' }}
      </button>

      <p class="ma-connection__note">
        Login, verificacao em duas etapas e consentimento continuam sendo feitos por voce na Meta. O
        Omni recebe o retorno e guarda o token cifrado automaticamente.
      </p>

      <p v-if="oauthError" class="ma-connection__oauth-error" role="alert">{{ oauthError }}</p>

      <p v-if="!store.canConnectMetaAds" class="ma-connection__readonly">
        Sua função possui acesso somente leitura e não pode conectar ou desconectar contas Meta.
      </p>

      <details class="ma-connection__manual">
        <summary class="ma-connection__manual-summary">Conexao manual avancada</summary>
        <form class="ma-connection__manual-form" @submit.prevent="onConnect">
          <label class="ma-connection__field">
            <span class="ma-connection__label">System User token</span>
            <textarea
              v-model="token"
              class="ma-connection__textarea"
              rows="4"
              spellcheck="false"
              autocomplete="off"
              placeholder="Cole aqui o token de longa duracao do Business Manager"
              :disabled="store.connecting || !store.canConnectMetaAds"
            ></textarea>
          </label>

          <p class="ma-connection__note">
            <span class="ma-connection__note-badge">Compatibilidade</span>
            Use somente se o Facebook Login ainda nao estiver configurado. O token e guardado
            cifrado.
          </p>

          <button
            type="submit"
            class="ma-connection__btn ma-connection__btn--ghost"
            :disabled="!canSubmitConnection"
            :title="
              store.canConnectMetaAds
                ? 'Salvar a conexão manual com a Meta'
                : 'Sua função não pode alterar conexões do Meta Ads'
            "
          >
            <span v-if="store.connecting" class="ma-connection__spinner" aria-hidden="true"></span>
            {{ store.connecting ? 'Conectando...' : 'Salvar token manual' }}
          </button>
        </form>
      </details>
    </div>
  </article>
</template>

<style scoped>
.ma-connection {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.ma-connection__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ma-connection__title {
  font-size: 1.3rem;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.ma-connection__subtitle {
  font-size: 0.88rem;
  color: var(--text-muted);
  margin-top: 0.25rem;
  max-width: 46ch;
}

.ma-connection__status {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  flex-shrink: 0;
  font-size: 0.8rem;
  font-weight: 600;
  padding: 0.35rem 0.75rem;
  border-radius: 999px;
  border: 1px solid var(--line-soft);
}

.ma-connection__status--on {
  color: rgb(var(--success));
  border-color: rgb(var(--success) / 0.4);
  background: rgb(var(--success) / 0.1);
}

.ma-connection__status--off {
  color: var(--text-muted);
  background: rgb(var(--surface-2) / 0.6);
}

.ma-connection__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

.ma-connection__divider {
  border: none;
  border-top: 1px solid var(--line-soft);
  margin: 0;
}

.ma-connection__connected {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1.25rem;
  flex-wrap: wrap;
}

.ma-connection__connected-actions {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.ma-connection__expiry {
  flex: 1 1 100%;
  margin: 0;
  padding: 0.65rem 0.75rem;
  border-radius: 0.55rem;
  font-size: 0.78rem;
}

.ma-connection__expiry--warning {
  color: rgb(var(--warning));
  background: rgb(var(--warning) / 0.1);
}

.ma-connection__expiry--danger {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.1);
}

.ma-connection__facts {
  display: flex;
  flex-wrap: wrap;
  gap: 1.75rem;
  margin: 0;
}

.ma-connection__fact {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.ma-connection__fact-label {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.ma-connection__fact-value {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-main);
  margin: 0;
}

.ma-connection__fact-value--ok {
  color: rgb(var(--success));
}

.ma-connection__fact-value--muted {
  color: var(--text-muted);
  max-width: 36ch;
}

.ma-connection__form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.ma-connection__field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.ma-connection__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.ma-connection__textarea {
  width: 100%;
  padding: 0.7rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.6rem;
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.85rem;
  line-height: 1.5;
  resize: vertical;
  min-height: 96px;
}

.ma-connection__textarea:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

.ma-connection__textarea:disabled {
  opacity: 0.6;
}

.ma-connection__note {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.5;
}

.ma-connection__readonly {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.ma-connection__oauth-error {
  font-size: 0.82rem;
  color: rgb(var(--danger));
}

.ma-connection__manual {
  border-top: 1px solid var(--line-soft);
  padding-top: 0.85rem;
}

.ma-connection__manual-summary {
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.82rem;
  font-weight: 600;
}

.ma-connection__manual-form {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  margin-top: 0.85rem;
}

.ma-connection__note-badge {
  flex-shrink: 0;
  font-size: 0.62rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  padding: 0.1rem 0.4rem;
  border-radius: 0.3rem;
  background: rgb(var(--primary) / 0.15);
  color: rgb(var(--primary));
}

.ma-connection__btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  align-self: flex-start;
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.6rem 1.3rem;
  border-radius: 0.55rem;
  cursor: pointer;
  border: 1px solid transparent;
}

.ma-connection__btn:disabled {
  opacity: 0.6;
  cursor: progress;
}

.ma-connection__btn--primary {
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.ma-connection__btn--ghost {
  border-color: var(--line-soft);
  color: var(--text-muted);
  background: transparent;
}

.ma-connection__btn--ghost:hover {
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
}

.ma-connection__spinner {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid rgb(255 255 255 / 0.4);
  border-top-color: rgb(255 255 255);
  animation: ma-connection-spin 0.7s linear infinite;
}

@keyframes ma-connection-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 720px) {
  .ma-connection__head {
    flex-direction: column;
  }
}
</style>
