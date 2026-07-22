<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  decideInstagramAction,
  fetchInstagramAccounts,
  fetchInstagramActions,
  fetchInstagramComments,
  saveInstagramAccount,
} from '~/domain/omnichannel/instagram-api'
import type {
  OmniInstagramAccount,
  OmniInstagramAction,
  OmniInstagramComment,
} from '~/domain/omnichannel/config-types'

const props = defineProps<{ canManage: boolean }>()
const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const accounts = ref<OmniInstagramAccount[]>([])
const comments = ref<OmniInstagramComment[]>([])
const actions = ref<OmniInstagramAction[]>([])
const actionDrafts = reactive<Record<string, string>>({})
const selectedComment = ref<OmniInstagramComment | null>(null)
const loading = ref(true)
const saving = ref(false)
const busy = ref(false)
const form = reactive({
  igUserId: '',
  username: '',
  displayName: '',
  pageId: '',
  graphVersion: 'v19.0',
  accessToken: '',
  appSecret: '',
  verifyToken: '',
})

const canSave = computed(
  () =>
    props.canManage &&
    !saving.value &&
    form.igUserId.trim() !== '' &&
    form.graphVersion.trim() !== '' &&
    form.accessToken.trim() !== '' &&
    form.appSecret.trim() !== '' &&
    form.verifyToken.trim() !== '',
)

async function load(): Promise<void> {
  loading.value = true
  try {
    const [nextAccounts, nextComments] = await Promise.all([
      fetchInstagramAccounts(api),
      fetchInstagramComments(api),
    ])
    accounts.value = nextAccounts
    comments.value = nextComments
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar o Instagram.'))
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  if (!canSave.value) return
  saving.value = true
  try {
    await saveInstagramAccount(api, { ...form })
    form.accessToken = ''
    form.appSecret = ''
    form.verifyToken = ''
    ui.success('Conta Instagram salva. As credenciais foram armazenadas com segurança.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar a conta Instagram.'))
  } finally {
    saving.value = false
  }
}

async function openComment(comment: OmniInstagramComment): Promise<void> {
  selectedComment.value = comment
  try {
    actions.value = await fetchInstagramActions(api, comment.id)
    for (const action of actions.value) {
      actionDrafts[action.id] = action.approvedText || action.proposedText || ''
    }
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar a moderação.'))
  }
}

async function decide(
  action: OmniInstagramAction,
  actionKind: OmniInstagramAction['actionKind'],
): Promise<void> {
  if (!selectedComment.value || !props.canManage || busy.value) return
  const text = actionDrafts[action.id] ?? action.approvedText ?? action.proposedText ?? ''
  busy.value = true
  try {
    await decideInstagramAction(api, selectedComment.value.id, action.id, {
      actionKind,
      approvedText: text,
    })
    ui.success(
      actionKind === 'ignore' ? 'Comentário ignorado.' : 'Ação aprovada e enfileirada pelo Go.',
    )
    await openComment(selectedComment.value)
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível decidir esta ação.'))
  } finally {
    busy.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="cfg-tab instagram-config">
    <p class="cfg-tab__lead">
      DMs entram no mesmo inbox. Comentários e menções são classificados pela IA, mas toda resposta
      pública/privada exige aprovação humana e sai somente pela outbox do Go.
    </p>

    <section class="ig-card">
      <div class="ig-card__head">
        <strong>Conta profissional</strong>
        <span>credenciais write-only</span>
      </div>
      <div class="ig-grid">
        <label class="cfg-field">
          <span class="cfg-field__label">IG user ID *</span>
          <input v-model="form.igUserId" class="cfg-input" :disabled="!canManage" />
        </label>
        <label class="cfg-field">
          <span class="cfg-field__label">Usuário</span>
          <input v-model="form.username" class="cfg-input" :disabled="!canManage" />
        </label>
        <label class="cfg-field">
          <span class="cfg-field__label">Page ID</span>
          <input v-model="form.pageId" class="cfg-input" :disabled="!canManage" />
        </label>
        <label class="cfg-field">
          <span class="cfg-field__label">Graph version *</span>
          <input v-model="form.graphVersion" class="cfg-input" :disabled="!canManage" />
        </label>
        <label class="cfg-field">
          <span class="cfg-field__label">Access token *</span>
          <input
            v-model="form.accessToken"
            class="cfg-input"
            type="password"
            autocomplete="new-password"
            :disabled="!canManage"
          />
        </label>
        <label class="cfg-field">
          <span class="cfg-field__label">App secret *</span>
          <input
            v-model="form.appSecret"
            class="cfg-input"
            type="password"
            autocomplete="new-password"
            :disabled="!canManage"
          />
        </label>
        <label class="cfg-field">
          <span class="cfg-field__label">Verify token *</span>
          <input
            v-model="form.verifyToken"
            class="cfg-input"
            type="password"
            autocomplete="new-password"
            :disabled="!canManage"
          />
        </label>
      </div>
      <div class="ig-card__foot">
        <AppPanelButton variant="primary" :disabled="!canSave" @click="save">
          Salvar conta
        </AppPanelButton>
      </div>
      <p v-if="accounts.length" class="ig-muted">
        {{ accounts.length }} conta(s) configurada(s); o painel nunca exibe tokens.
      </p>
    </section>

    <section class="ig-card">
      <div class="ig-card__head">
        <strong>Comentários e menções</strong>
        <span>{{ comments.length }} pendentes/recebidos</span>
      </div>
      <p v-if="loading" class="ig-muted">Carregando…</p>
      <p v-else-if="!comments.length" class="ig-muted">Nenhum comentário recebido ainda.</p>
      <button
        v-for="comment in comments"
        v-else
        :key="comment.id"
        type="button"
        class="ig-comment"
        @click="openComment(comment)"
      >
        <span>
          <strong>@{{ comment.username || comment.authorScopedId }}</strong>
          · {{ comment.text }}
        </span>
        <small>{{ comment.status }}</small>
      </button>
    </section>

    <section v-if="selectedComment" class="ig-card">
      <div class="ig-card__head">
        <strong>
          Moderação: @{{ selectedComment.username || selectedComment.authorScopedId }}
        </strong>
        <span>{{ selectedComment.text }}</span>
      </div>
      <div v-for="action in actions" :key="action.id" class="ig-action">
        <div class="ig-action__copy">
          <strong>{{ action.actionKind }}</strong>
          <small>{{ action.status }} · {{ action.proposedText || 'sem rascunho' }}</small>
          <textarea
            v-if="
              action.status === 'pending_review' &&
              (action.actionKind === 'public_reply' || action.actionKind === 'private_reply')
            "
            v-model="actionDrafts[action.id]"
            class="cfg-input ig-action__textarea"
            maxlength="4000"
            :disabled="!canManage || busy"
          ></textarea>
        </div>
        <div class="ig-action__buttons">
          <AppPanelButton
            v-if="action.status === 'pending_review'"
            variant="primary"
            :disabled="busy || !canManage"
            @click="decide(action, action.actionKind)"
          >
            Aprovar
          </AppPanelButton>
          <AppPanelButton
            v-if="action.status === 'pending_review'"
            variant="ghost"
            :disabled="busy || !canManage"
            @click="decide(action, 'ignore')"
          >
            Ignorar
          </AppPanelButton>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.instagram-config {
  display: grid;
  gap: 0.75rem;
}
.ig-card {
  display: grid;
  gap: 0.75rem;
  padding: 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
}
.ig-card__head,
.ig-card__foot,
.ig-action,
.ig-comment {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}
.ig-card__head span,
.ig-muted,
.ig-action small,
.ig-comment small {
  color: rgb(var(--muted));
  font-size: 0.76rem;
}
.ig-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 0.65rem;
}
.ig-comment {
  width: 100%;
  padding: 0.65rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: transparent;
  color: rgb(var(--text));
  text-align: left;
  cursor: pointer;
}
.ig-comment:hover {
  border-color: rgb(var(--primary) / 0.5);
}
.ig-action {
  padding-top: 0.65rem;
  border-top: 1px solid var(--line-soft);
}
.ig-action > div:first-child {
  display: grid;
  gap: 0.2rem;
}
.ig-action__copy {
  min-width: 0;
  flex: 1;
}
.ig-action__textarea {
  width: 100%;
  min-height: 68px;
  padding: 0.5rem;
}
.ig-action__buttons {
  display: flex;
  gap: 0.4rem;
}
</style>
