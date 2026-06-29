<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'

import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppPasswordInput from '~/components/ui/AppPasswordInput.vue'
import ProfileAccessCard from '~/components/profile/ProfileAccessCard.vue'
import ProfileSecurityCard from '~/components/profile/ProfileSecurityCard.vue'
import ProfileStoresCard from '~/components/profile/ProfileStoresCard.vue'
import ProfileFeedbackCard from '~/components/profile/ProfileFeedbackCard.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { getApiBase, getApiErrorMessage } from '~/utils/api-client'

definePageMeta({
  layout: 'dashboard',
  workspaceId: '',
  pageLabel: 'Perfil',
})

const runtimeConfig = useRuntimeConfig()
const auth = useAuthStore()
const ui = useUiStore()

onMounted(() => {
  void auth.ensureSession()
})

const profileDraft = reactive({
  displayName: '',
  email: '',
})
const passwordDraft = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
})
const avatarPending = ref(false)
const profilePending = ref(false)
const passwordPending = ref(false)

// Feedback inline do formulario de senha (regra: dizer o que falta, sem botao morto).
const passwordTooShort = computed(
  () => Boolean(passwordDraft.newPassword) && String(passwordDraft.newPassword).trim().length < 8,
)
const passwordsMismatch = computed(
  () =>
    Boolean(passwordDraft.confirmPassword) &&
    passwordDraft.newPassword !== passwordDraft.confirmPassword,
)

// Indicador de forca da senha (so visual; o gate real e min 8 + confirmacao).
const passwordStrength = computed(() => {
  const value = String(passwordDraft.newPassword || '')
  if (!value) {
    return { score: 0, label: '' }
  }
  let score = 0
  if (value.length >= 8) score += 1
  if (value.length >= 12) score += 1
  if (/[a-z]/.test(value) && /[A-Z]/.test(value)) score += 1
  if (/\d/.test(value)) score += 1
  if (/[^A-Za-z0-9]/.test(value)) score += 1
  const normalized = Math.min(4, Math.max(1, score))
  const label =
    normalized <= 1 ? 'fraca' : normalized === 2 ? 'media' : normalized === 3 ? 'boa' : 'forte'
  return { score: normalized, label }
})

const avatarUrl = computed(() => {
  const avatarPath = String(auth.user?.avatarPath || '').trim()
  if (!avatarPath) {
    return ''
  }

  return new URL(avatarPath, getApiBase(runtimeConfig)).toString()
})

const initials = computed(() =>
  String(auth.user?.displayName || '')
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((chunk) => chunk[0] || '')
    .join('')
    .toUpperCase(),
)

watch(
  () => auth.user,
  (user) => {
    profileDraft.displayName = String(user?.displayName || '')
    profileDraft.email = String(user?.email || '')
  },
  {
    immediate: true,
    deep: true,
  },
)

async function saveProfile() {
  profilePending.value = true

  try {
    await auth.updateProfile(profileDraft)
    ui.success('Perfil atualizado.')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Nao foi possivel atualizar o perfil.'))
  } finally {
    profilePending.value = false
  }
}

async function changePassword() {
  if (String(passwordDraft.newPassword || '').trim().length < 8) {
    ui.error('A nova senha deve ter pelo menos 8 caracteres.')
    return
  }

  if (passwordDraft.newPassword !== passwordDraft.confirmPassword) {
    ui.error('A confirmacao da senha nao confere.')
    return
  }

  passwordPending.value = true

  try {
    await auth.changePassword(passwordDraft)
    passwordDraft.currentPassword = ''
    passwordDraft.newPassword = ''
    passwordDraft.confirmPassword = ''
    ui.success('Senha alterada.')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Nao foi possivel alterar a senha.'))
  } finally {
    passwordPending.value = false
  }
}

async function handleAvatarChange(event) {
  const file = event?.target?.files?.[0] || null
  if (!file) {
    return
  }

  avatarPending.value = true

  try {
    await auth.uploadAvatar(file)
    ui.success('Foto atualizada.')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Nao foi possivel enviar a foto.'))
  } finally {
    avatarPending.value = false
    event.target.value = ''
  }
}
</script>

<template>
  <div class="page-workspace">
    <section class="admin-panel profile-panel" data-testid="profile-panel">
      <header class="admin-panel__header">
        <h2 class="admin-panel__title">Meu perfil</h2>
        <p class="admin-panel__text">
          Atualize sua foto, nome, email e senha sem depender do administrativo.
        </p>
      </header>

      <article v-if="auth.mustChangePassword" class="insight-card">
        <p class="settings-card__text">
          Sua conta ainda está com senha temporária. Antes de continuar usando a plataforma,
          atualize sua senha abaixo.
        </p>
      </article>

      <div class="profile-panel__layout">
        <div class="profile-panel__col profile-panel__col--side">
          <article class="settings-card profile-panel__avatar-card">
            <header class="settings-card__header">
              <h3 class="settings-card__title">Foto</h3>
              <p class="settings-card__text">JPG, PNG ou WebP com ate 2 MB.</p>
            </header>

            <div class="profile-panel__avatar-wrap">
              <img
                v-if="avatarUrl"
                :src="avatarUrl"
                alt="Foto do usuario"
                class="profile-panel__avatar-image"
              />
              <span v-else class="profile-panel__avatar-fallback">{{ initials || 'US' }}</span>
            </div>

            <AppPanelButton
              as="label"
              block
              class="profile-panel__avatar-button"
              :disabled="avatarPending"
            >
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                hidden
                @change="handleAvatarChange"
              />
              {{ avatarPending ? 'Enviando...' : 'Enviar nova foto' }}
            </AppPanelButton>
          </article>

          <ProfileAccessCard />
          <ProfileSecurityCard />
        </div>

        <div class="profile-panel__col profile-panel__col--main">
          <article class="settings-card">
            <header class="settings-card__header">
              <h3 class="settings-card__title">Dados pessoais</h3>
              <p class="settings-card__text">
                Esses dados aparecem na conta e nas areas autenticadas.
              </p>
            </header>

            <form class="multistore-form multistore-form--add" @submit.prevent="saveProfile">
              <div class="multistore-form__row">
                <input
                  v-model="profileDraft.displayName"
                  class="product-add__input"
                  type="text"
                  placeholder="Nome completo *"
                />
                <input
                  v-model="profileDraft.email"
                  class="product-add__input"
                  type="email"
                  placeholder="Email *"
                />
              </div>
              <div class="multistore-form__actions">
                <AppPanelButton type="submit" :disabled="profilePending">
                  {{ profilePending ? 'Salvando...' : 'Salvar perfil' }}
                </AppPanelButton>
              </div>
            </form>
          </article>

          <article class="settings-card">
            <header class="settings-card__header">
              <h3 class="settings-card__title">Senha</h3>
              <p class="settings-card__text">
                {{
                  auth.mustChangePassword
                    ? 'Defina agora sua senha pessoal para liberar o restante do sistema.'
                    : 'Troque sua senha mantendo a conta protegida.'
                }}
              </p>
            </header>

            <form class="multistore-form multistore-form--add" @submit.prevent="changePassword">
              <div class="multistore-form__row multistore-form__row--password">
                <AppPasswordInput
                  v-model="passwordDraft.currentPassword"
                  autocomplete="current-password"
                  :placeholder="
                    auth.mustChangePassword ? 'Senha temporaria atual *' : 'Senha atual *'
                  "
                />
                <AppPasswordInput
                  v-model="passwordDraft.newPassword"
                  autocomplete="new-password"
                  placeholder="Nova senha *"
                />
                <AppPasswordInput
                  v-model="passwordDraft.confirmPassword"
                  autocomplete="new-password"
                  placeholder="Confirmar nova senha *"
                />
              </div>
              <div
                v-if="passwordStrength.score > 0"
                class="profile-panel__strength"
                :class="`profile-panel__strength--${passwordStrength.score}`"
              >
                <span class="profile-panel__strength-track">
                  <span
                    v-for="n in 4"
                    :key="n"
                    class="profile-panel__strength-seg"
                    :class="{ 'is-on': n <= passwordStrength.score }"
                  ></span>
                </span>
                <span class="profile-panel__strength-label">
                  Forca: {{ passwordStrength.label }}
                </span>
              </div>
              <p
                v-if="passwordTooShort"
                class="profile-panel__hint profile-panel__hint--warn"
                role="status"
              >
                A nova senha precisa de pelo menos 8 caracteres.
              </p>
              <p
                v-else-if="passwordsMismatch"
                class="profile-panel__hint profile-panel__hint--warn"
                role="status"
              >
                A confirmacao nao confere com a nova senha.
              </p>
              <p v-else class="profile-panel__hint">
                Use ao menos 8 caracteres. Campos com * sao obrigatorios.
              </p>
              <div class="multistore-form__actions">
                <AppPanelButton type="submit" :disabled="passwordPending">
                  {{
                    passwordPending
                      ? 'Atualizando...'
                      : auth.mustChangePassword
                        ? 'Definir minha senha'
                        : 'Atualizar senha'
                  }}
                </AppPanelButton>
              </div>
            </form>
          </article>

          <ProfileStoresCard />
          <ProfileFeedbackCard />
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.profile-panel__avatar-button {
  margin-top: auto;
}

.profile-panel__hint {
  margin: 0.1rem 0 0;
  font-size: 0.78rem;
  line-height: 1.45;
  color: var(--text-muted);
}

.profile-panel__hint--warn {
  color: var(--accent-warning);
}

/* 2 colunas usando a largura TOTAL da tela (sem max-width): lateral compacta
   (Foto/acesso/seguranca) + principal (dados/senha/lojas/chamados). */
.profile-panel__layout {
  display: grid;
  grid-template-columns: minmax(280px, 340px) minmax(0, 1fr);
  gap: 1rem;
  align-items: start;
  margin-top: 0.4rem;
}

.profile-panel__col {
  display: grid;
  gap: 1rem;
  align-content: start;
  min-width: 0;
}

@media (max-width: 880px) {
  .profile-panel__layout {
    grid-template-columns: 1fr;
  }
}

/* Campos do form com largura legivel: ficam lado a lado, mas o form nao estica na
   coluna larga (cap de 720px). Override scoped — so este perfil. */
.multistore-form__row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 0.6rem;
  max-width: 720px;
}

.profile-panel__strength {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  margin-top: 0.2rem;
}

.profile-panel__strength-track {
  display: inline-flex;
  gap: 0.25rem;
}

.profile-panel__strength-seg {
  width: 2rem;
  height: 0.32rem;
  border-radius: 999px;
  background: var(--line-soft);
}

.profile-panel__strength-seg.is-on {
  background: var(--accent-warning);
}

.profile-panel__strength--3 .profile-panel__strength-seg.is-on,
.profile-panel__strength--4 .profile-panel__strength-seg.is-on {
  background: var(--accent-success);
}

.profile-panel__strength-label {
  font-size: 0.76rem;
  color: var(--text-muted);
  text-transform: capitalize;
}
</style>
