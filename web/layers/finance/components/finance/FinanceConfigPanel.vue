<script setup lang="ts">
import {
  OmniInputsOmniMoneyInput as OmniMoneyInput,
  InputsOmniSelectMenuInput as OmniSelectMenuInput,
} from '#components'
// Slideover de configuracao financeira — port fiel do web-reference (mesma
// estrutura, classes e estilos). Injeta o editor de config.
import { FINANCE_CONFIG_KEY } from '../../composables/useFinanceConfigEditor'
import {
  formatMoney,
  formatRecurringStoreBreakdown,
  KIND_OPTIONS,
} from '../../utils/finance-helpers'

import FinanceAssistantConfig from './FinanceAssistantConfig.vue'
import FinanceReceiptsConfig from './FinanceReceiptsConfig.vue'
import FinanceRetentionConfig from './FinanceRetentionConfig.vue'

const config = inject(FINANCE_CONFIG_KEY)!

const {
  configOpen,
  configDraft,
  clientRecurringEntries,
  canManageConfig,
  canManageSheets,
  canManageRecurring,
  saveLabel,
  retrySave,
  errorMessage: configErrorMessage,
  editingFixedId,
  fixedEditDraft,
  newFixed,
  categoryConfigOptions,
  resolveCategoryNameById,
  recurringEntryForTenant,
  queueConfigPersist,
  addFixedAccount,
  startEditFixed,
  finishEditFixed,
  cancelEditFixed,
  removeFixedAccount,
  addFixedMember,
  removeFixedMember,
  updateFixedAmountFromMembers,
  setRecurringAdjustment,
  setRecurringNotes,
} = config
</script>

<template>
  <USlideover
    v-model:open="configOpen"
    class="finances-config__slideover"
    side="right"
    title="Configuracao financeira"
    description="Cadastre categorias e contas fixas para acelerar o preenchimento das linhas."
    :ui="{ content: 'max-w-2xl' }"
  >
    <template #body>
      <UButton to="/perfil#fiscal" icon="i-lucide-file-text" color="neutral" variant="soft">
        Dados fiscais e templates no meu perfil
      </UButton>
      <FinanceAssistantConfig :can-manage="canManageConfig" />
      <FinanceReceiptsConfig :can-manage="canManageSheets" />
      <FinanceRetentionConfig :can-manage="canManageConfig" />
      <fieldset :disabled="!canManageConfig" class="finances-config space-y-4">
        <UAlert
          v-if="configErrorMessage"
          class="finances-config__alert finances-config__alert--error"
          color="error"
          variant="soft"
          icon="i-lucide-alert-triangle"
          title="Erro"
          :description="configErrorMessage"
        />

        <UCollapsible :default-open="false" class="finances-config__collapsible">
          <template #default="{ open }">
            <div class="finances-config__collapsible-trigger">
              <div class="min-w-0">
                <p class="text-sm font-semibold">Entradas de clientes</p>
                <p class="text-xs text-[rgb(var(--muted))]">
                  Mensalidades ativas que alimentam as entradas.
                </p>
              </div>
              <div class="flex items-center gap-2">
                <UBadge class="finances-config__counter-badge" color="neutral" variant="soft">
                  {{ clientRecurringEntries.length }} itens
                </UBadge>
                <UIcon
                  :name="open ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
                  class="size-4"
                />
              </div>
            </div>
          </template>

          <template #content>
            <div class="finances-config__collapsible-content space-y-2">
              <div
                v-if="clientRecurringEntries.length === 0"
                class="rounded-[var(--radius-sm)] border border-dashed border-[rgb(var(--border))] p-3 text-xs text-[rgb(var(--muted))]"
              >
                Nenhuma mensalidade ativa encontrada para o cliente atual.
              </div>
              <div
                v-for="entry in clientRecurringEntries"
                :key="entry.id"
                class="finances-config__item-row"
              >
                <div class="min-w-0 w-full space-y-1">
                  <div class="flex items-center justify-between gap-2">
                    <p class="truncate text-sm font-medium">{{ entry.name }}</p>
                    <UBadge
                      class="finances-config__item-total-badge"
                      color="success"
                      variant="soft"
                    >
                      {{
                        formatMoney(
                          entry.amount +
                            Number(recurringEntryForTenant(entry.id)?.adjustmentAmount || 0),
                        )
                      }}
                    </UBadge>
                  </div>
                  <p class="text-xs text-[rgb(var(--muted))]">
                    Base: {{ formatMoney(entry.amount) }} | Vencimento: {{ entry.dueDay || '--' }}
                  </p>
                  <p
                    v-if="entry.billingMode === 'per_store' && entry.stores.length > 0"
                    class="text-xs text-[rgb(var(--muted))]"
                  >
                    Lojas: {{ formatRecurringStoreBreakdown(entry) }}
                  </p>
                  <div class="grid gap-2 md:grid-cols-[140px_minmax(0,1fr)]">
                    <UInput
                      class="finances-config__input finances-config__input--recurring-adjustment"
                      :disabled="!canManageRecurring"
                      :model-value="
                        Number(recurringEntryForTenant(entry.id)?.adjustmentAmount || 0)
                      "
                      type="number"
                      step="0.01"
                      placeholder="+500 ou -1000"
                      @update:model-value="setRecurringAdjustment(entry.id, Number($event || 0))"
                    />
                    <UInput
                      class="finances-config__input finances-config__input--recurring-notes"
                      :disabled="!canManageRecurring"
                      :model-value="recurringEntryForTenant(entry.id)?.notes || ''"
                      placeholder="Motivo do ajuste do mes"
                      @update:model-value="setRecurringNotes(entry.id, String($event || ''))"
                    />
                  </div>
                </div>
              </div>
            </div>
          </template>
        </UCollapsible>

        <FinanceCategoryConfig />

        <UCollapsible :default-open="false" class="finances-config__collapsible">
          <template #default="{ open }">
            <div class="finances-config__collapsible-trigger">
              <div class="min-w-0">
                <p class="text-sm font-semibold">Contas fixas</p>
                <p class="text-xs text-[rgb(var(--muted))]">
                  Cadastre custos/receitas recorrentes e composicao detalhada.
                </p>
              </div>
              <div class="flex items-center gap-2">
                <UBadge class="finances-config__counter-badge" color="neutral" variant="soft">
                  {{ configDraft.fixedAccounts.length }} itens
                </UBadge>
                <UIcon
                  :name="open ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
                  class="size-4"
                />
              </div>
            </div>
          </template>

          <template #content>
            <div class="finances-config__collapsible-content space-y-2">
              <div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_130px_170px]">
                <UInput
                  v-model="newFixed.name"
                  class="finances-config__input finances-config__input--fixed-name"
                  placeholder="Nome da conta fixa"
                />
                <OmniSelectMenuInput
                  v-model="newFixed.kind"
                  class="finances-config__input finances-config__input--fixed-kind"
                  :items="KIND_OPTIONS"
                  item-display-mode="text"
                  :searchable="false"
                  :creatable="false"
                  :clear="false"
                  :badge-mode="true"
                  option-edit-mode="none"
                  color="neutral"
                  variant="none"
                  :highlight="false"
                />
                <OmniSelectMenuInput
                  v-model="newFixed.categoryId"
                  :items="categoryConfigOptions"
                  placeholder="Categoria"
                  searchable
                  clear
                  item-display-mode="text"
                  :badge-mode="true"
                  option-edit-mode="color"
                  color="neutral"
                  variant="none"
                  :highlight="false"
                />
              </div>
              <div class="grid gap-2 md:grid-cols-[180px_minmax(0,1fr)]">
                <OmniMoneyInput v-model="newFixed.defaultAmount" />
                <UInput
                  v-model="newFixed.notes"
                  class="finances-config__input finances-config__input--fixed-notes"
                  placeholder="Observacao (opcional)"
                />
              </div>
              <div class="flex justify-end">
                <UButton
                  class="finances-config__action finances-config__action--add-fixed"
                  icon="i-lucide-plus"
                  size="sm"
                  @click="addFixedAccount"
                >
                  Adicionar conta fixa
                </UButton>
              </div>
            </div>

            <div class="mt-3 space-y-3">
              <UCollapsible
                v-for="account in configDraft.fixedAccounts"
                :key="account.id"
                class="finances-config__fixed-collapsible"
                :default-open="false"
              >
                <template #default="{ open }">
                  <div class="finances-config__collapsible-trigger">
                    <div class="min-w-0">
                      <p class="truncate text-sm font-semibold">{{ account.name }}</p>
                      <p class="text-xs text-[rgb(var(--muted))]">
                        {{ formatMoney(account.defaultAmount) }}
                      </p>
                    </div>
                    <div class="flex items-center gap-1">
                      <UBadge
                        class="finances-config__fixed-badge finances-config__fixed-badge--count"
                        color="neutral"
                        variant="soft"
                        size="xs"
                      >
                        {{ account.members.length }} itens
                      </UBadge>
                      <UBadge
                        class="finances-config__fixed-badge finances-config__fixed-badge--kind"
                        color="neutral"
                        variant="soft"
                        size="xs"
                      >
                        {{ account.kind }}
                      </UBadge>
                      <UBadge
                        class="finances-config__fixed-badge finances-config__fixed-badge--category"
                        color="neutral"
                        variant="soft"
                        size="xs"
                      >
                        {{ resolveCategoryNameById(account.categoryId) || 'Sem categoria' }}
                      </UBadge>
                      <UButton
                        icon="i-lucide-pencil"
                        color="neutral"
                        variant="ghost"
                        size="xs"
                        class="finances-config__action finances-config__action--edit-fixed"
                        @click.stop="startEditFixed(account.id)"
                      />
                      <UButton
                        icon="i-lucide-trash-2"
                        color="error"
                        variant="ghost"
                        size="xs"
                        class="finances-config__action finances-config__action--remove-fixed"
                        @click.stop="removeFixedAccount(account.id)"
                      />
                      <UIcon
                        :name="open ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
                        class="size-4"
                      />
                    </div>
                  </div>
                </template>

                <template #content>
                  <div class="finances-config__collapsible-content space-y-2">
                    <template v-if="editingFixedId === account.id">
                      <div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_130px_170px]">
                        <UInput
                          v-model="fixedEditDraft.name"
                          class="finances-config__input finances-config__input--fixed-edit-name"
                          placeholder="Nome da conta fixa"
                        />
                        <OmniSelectMenuInput
                          v-model="fixedEditDraft.kind"
                          class="finances-config__input finances-config__input--fixed-edit-kind"
                          :items="KIND_OPTIONS"
                          item-display-mode="text"
                          :searchable="false"
                          :creatable="false"
                          :clear="false"
                          :badge-mode="true"
                          option-edit-mode="none"
                          color="neutral"
                          variant="none"
                          :highlight="false"
                        />
                        <OmniSelectMenuInput
                          v-model="fixedEditDraft.categoryId"
                          :items="categoryConfigOptions"
                          placeholder="Categoria"
                          searchable
                          clear
                          item-display-mode="text"
                          :badge-mode="true"
                          option-edit-mode="color"
                          color="neutral"
                          variant="none"
                          :highlight="false"
                        />
                      </div>
                      <div class="flex justify-end gap-1">
                        <UButton
                          size="xs"
                          color="neutral"
                          variant="soft"
                          class="finances-config__action finances-config__action--save-fixed"
                          @click="finishEditFixed"
                        >
                          Salvar
                        </UButton>
                        <UButton
                          size="xs"
                          color="neutral"
                          variant="ghost"
                          class="finances-config__action finances-config__action--cancel-fixed"
                          @click="cancelEditFixed"
                        >
                          Cancelar
                        </UButton>
                      </div>
                    </template>

                    <div class="grid gap-2 md:grid-cols-[180px_minmax(0,1fr)]">
                      <OmniMoneyInput
                        v-model="account.defaultAmount"
                        @update:model-value="queueConfigPersist"
                      />
                      <UInput
                        v-model="account.notes"
                        class="finances-config__input finances-config__input--fixed-account-notes"
                        placeholder="Observacao da conta fixa"
                        @update:model-value="queueConfigPersist"
                      />
                    </div>

                    <div class="finances-config__details">
                      <p class="finances-config__details-summary">
                        Composicao (ex.: folha salarial)
                      </p>
                      <div class="space-y-2 pt-2">
                        <div
                          v-for="member in account.members"
                          :key="member.id"
                          class="grid gap-2 md:grid-cols-[minmax(0,1fr)_160px_auto]"
                        >
                          <UInput
                            v-model="member.name"
                            class="finances-config__input finances-config__input--fixed-member-name"
                            placeholder="Pessoa/item"
                            @update:model-value="queueConfigPersist"
                          />
                          <OmniMoneyInput
                            v-model="member.amount"
                            @update:model-value="
                              updateFixedAmountFromMembers(account, {
                                preserveWhenEmpty: true,
                                persist: true,
                              })
                            "
                          />
                          <UButton
                            icon="i-lucide-x"
                            color="error"
                            variant="ghost"
                            size="xs"
                            class="finances-config__action finances-config__action--remove-fixed-member"
                            @click="removeFixedMember(account, member.id)"
                          />
                        </div>
                        <div class="flex justify-start">
                          <UButton
                            icon="i-lucide-plus"
                            size="xs"
                            color="neutral"
                            variant="soft"
                            class="finances-config__action finances-config__action--add-fixed-member"
                            @click.stop="addFixedMember(account)"
                          >
                            Adicionar item
                          </UButton>
                        </div>
                      </div>
                    </div>
                  </div>
                </template>
              </UCollapsible>
            </div>
          </template>
        </UCollapsible>
      </fieldset>
    </template>

    <template #footer>
      <div class="flex w-full items-center justify-between gap-2">
        <p class="text-xs text-[rgb(var(--muted))]">
          {{ saveLabel }}
        </p>
        <UButton
          v-if="saveLabel.includes('Falha')"
          color="error"
          variant="soft"
          @click="void retrySave()"
        >
          Tentar novamente
        </UButton>
        <UButton
          color="neutral"
          variant="ghost"
          class="finances-config__action finances-config__action--close"
          @click="configOpen = false"
        >
          Fechar
        </UButton>
      </div>
    </template>
  </USlideover>
</template>

<style>
.finances-config .omni-select-menu-input {
  display: flex;
}

.finances-config__item-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 8px 10px;
}

.finances-config__collapsible {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 10px;
}

.finances-config__collapsible-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  cursor: pointer;
}

.finances-config__collapsible-content {
  padding-top: 10px;
}

.finances-config__fixed-card {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 10px;
}

.finances-config__details {
  border-top: 1px dashed rgb(var(--border));
  padding-top: 8px;
}

.finances-config__details-summary {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  color: rgb(var(--muted));
}
</style>
