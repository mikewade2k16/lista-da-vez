<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import {
  createIntelligenceSourceDraft,
  SOURCE_STATUS_OPTIONS,
  validateIntelligenceSourceDraft,
  type IntelligenceSourceConfig,
  type IntelligenceSourceDescriptor,
  type IntelligenceSourceDraft,
  type SourceConfigField,
  type SourceConfigValue,
} from '~/domain/customer-intelligence/sources'

const props = defineProps<{
  open: boolean
  descriptor: IntelligenceSourceDescriptor | null
  source: IntelligenceSourceConfig | null
  saving: boolean
  canManage: boolean
}>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  save: [draft: IntelligenceSourceDraft]
}>()

const draft = ref<IntelligenceSourceDraft | null>(null)
const attempted = ref(false)
const validation = computed(() => {
  if (!props.descriptor || !draft.value) {
    return { valid: false, errors: {} as Record<string, string> }
  }
  return validateIntelligenceSourceDraft(props.descriptor, draft.value)
})
const firstError = computed(() => Object.values(validation.value.errors)[0] ?? '')
const retentionDays = computed({
  get: () => Math.max(1, Math.round((draft.value?.snapshotTtlSeconds ?? 86_400) / 86_400)),
  set: (value: number) => {
    if (draft.value) draft.value.snapshotTtlSeconds = Number(value) * 86_400
  },
})

watch(
  () =>
    [props.open, props.descriptor?.sourceKey, props.source?.id, props.source?.revision] as const,
  () => {
    if (!props.open || !props.descriptor) return
    draft.value = createIntelligenceSourceDraft(props.descriptor, props.source)
    attempted.value = false
  },
  { immediate: true },
)

function errorFor(key: string): string {
  return attempted.value ? (validation.value.errors[key] ?? '') : ''
}

function configValue(key: string): SourceConfigValue | undefined {
  return draft.value?.config[key]
}

function stringConfigValue(key: string): string {
  const value = configValue(key)
  return typeof value === 'string' ? value : ''
}

function booleanConfigValue(key: string): boolean {
  return configValue(key) === true
}

function listConfigValue(key: string): string[] {
  const value = configValue(key)
  return Array.isArray(value) ? value : []
}

function setConfigValue(key: string, value: SourceConfigValue | undefined): void {
  if (!draft.value) return
  const config = { ...draft.value.config }
  if (value === undefined) Reflect.deleteProperty(config, key)
  else config[key] = value
  draft.value.config = config
}

function setTextConfig(key: string, event: Event): void {
  const value = (event.target as HTMLInputElement).value
  setConfigValue(key, value || undefined)
}

function setIntegerConfig(key: string, event: Event): void {
  const value = (event.target as HTMLInputElement).value
  setConfigValue(key, value === '' ? undefined : Number(value))
}

function setSelectConfig(key: string, value: string): void {
  setConfigValue(key, value || undefined)
}

function toggleConfigList(key: string, value: string, checked: boolean): void {
  const values = listConfigValue(key)
  setConfigValue(
    key,
    checked
      ? [...values.filter((item) => item !== value), value]
      : values.filter((item) => item !== value),
  )
}

function toggleAllowedField(field: string, checked: boolean): void {
  if (!draft.value) return
  draft.value.fieldAllowlist = checked
    ? [...draft.value.fieldAllowlist.filter((item) => item !== field), field]
    : draft.value.fieldAllowlist.filter((item) => item !== field)
}

function inputPattern(field: SourceConfigField): string | undefined {
  if (field.type === 'safe_key') {
    return '[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*'
  }
  if (field.type === 'uuid') {
    return '[0-9a-fA-F-]{36}'
  }
  return undefined
}

function submit(): void {
  attempted.value = true
  if (!draft.value || !validation.value.valid || !props.canManage || props.saving) {
    return
  }
  emit('save', {
    ...draft.value,
    fieldAllowlist: [...draft.value.fieldAllowlist],
    config: Object.fromEntries(
      Object.entries(draft.value.config).map(([key, value]) => [
        key,
        Array.isArray(value) ? [...value] : value,
      ]),
    ),
  })
}
</script>

<template>
  <OmniEntityDrawer
    :model-value="open"
    :title="
      source ? descriptor?.name || 'Configurar fonte' : `Criar ${descriptor?.name || 'fonte'}`
    "
    :subtitle="descriptor?.description"
    @update:model-value="emit('update:open', $event)"
  >
    <form v-if="draft && descriptor" class="source-form" @submit.prevent="submit">
      <p v-if="!source" class="source-form__privacy">
        Privacidade por padrao: a fonte nasce desabilitada e sem nenhum campo liberado.
      </p>

      <section class="source-form__grid">
        <label>
          <span>Chave da conexao</span>
          <input
            v-model="draft.connectionKey"
            type="text"
            maxlength="120"
            pattern="[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*"
            autocomplete="off"
            :disabled="!canManage || Boolean(source)"
          />
          <small v-if="source">
            A chave identifica esta configuracao e nao pode ser renomeada.
          </small>
          <small v-if="errorFor('connectionKey')" class="source-form__error">
            {{ errorFor('connectionKey') }}
          </small>
        </label>

        <AppSelectField
          v-model="draft.status"
          label="Status"
          :options="[...SOURCE_STATUS_OPTIONS]"
          :disabled="!canManage"
        />
        <AppSelectField
          v-model="draft.mode"
          label="Modo de coleta"
          :options="descriptor.modes.map((item) => ({ value: item, label: item }))"
          :disabled="!canManage"
        />
        <AppSelectField
          v-model="draft.purposeKey"
          label="Finalidade"
          :options="descriptor.purposeKeys.map((item) => ({ value: item, label: item }))"
          :disabled="!canManage"
        />

        <label>
          <span>Freshness (segundos)</span>
          <input
            v-model.number="draft.freshnessSeconds"
            type="number"
            min="0"
            max="31536000"
            step="1"
            :disabled="!canManage"
          />
          <small v-if="errorFor('freshnessSeconds')" class="source-form__error">
            {{ errorFor('freshnessSeconds') }}
          </small>
        </label>
      </section>

      <fieldset class="source-form__section">
        <legend>Campos liberados para a IA</legend>
        <p>Somente os campos marcados podem sair do modulo de origem e entrar nas observacoes.</p>
        <div v-if="descriptor.allowedFields.length" class="source-form__checks">
          <label v-for="field in descriptor.allowedFields" :key="field">
            <input
              type="checkbox"
              :checked="draft.fieldAllowlist.includes(field)"
              :disabled="!canManage"
              @change="toggleAllowedField(field, ($event.target as HTMLInputElement).checked)"
            />
            <span>{{ field }}</span>
          </label>
        </div>
        <p v-else>Esta fonte nao publica campos individuais.</p>
        <small v-if="errorFor('fieldAllowlist')" class="source-form__error">
          {{ errorFor('fieldAllowlist') }}
        </small>
      </fieldset>

      <fieldset v-if="descriptor.configSchema.length" class="source-form__section">
        <legend>Configuracao registrada</legend>
        <p>Somente valores tipados pelo descriptor sao enviados ao servidor.</p>

        <div v-for="field in descriptor.configSchema" :key="field.key" class="source-form__config">
          <label v-if="field.type === 'integer'">
            <span>{{ field.label }}{{ field.required ? ' *' : '' }}</span>
            <input
              :value="configValue(field.key) ?? ''"
              type="number"
              step="1"
              :min="field.min"
              :max="field.max"
              :disabled="!canManage"
              @input="setIntegerConfig(field.key, $event)"
            />
          </label>

          <label v-else-if="field.type === 'boolean'" class="source-form__check">
            <input
              type="checkbox"
              :checked="booleanConfigValue(field.key)"
              :disabled="!canManage"
              @change="setConfigValue(field.key, ($event.target as HTMLInputElement).checked)"
            />
            <span>{{ field.label }}</span>
          </label>

          <AppSelectField
            v-else-if="field.type === 'select'"
            :model-value="stringConfigValue(field.key)"
            :label="`${field.label}${field.required ? ' *' : ''}`"
            :options="[
              ...(field.required ? [] : [{ value: '', label: 'Nao definido' }]),
              ...field.options.map((item) => ({ value: item, label: item })),
            ]"
            :disabled="!canManage"
            @update:model-value="setSelectConfig(field.key, $event)"
          />

          <fieldset v-else-if="field.type === 'string_list'" class="source-form__nested">
            <legend>{{ field.label }}{{ field.required ? ' *' : '' }}</legend>
            <label v-for="item in field.elementKeys" :key="item">
              <input
                type="checkbox"
                :checked="listConfigValue(field.key).includes(item)"
                :disabled="!canManage"
                @change="
                  toggleConfigList(field.key, item, ($event.target as HTMLInputElement).checked)
                "
              />
              <span>{{ item }}</span>
            </label>
          </fieldset>

          <label v-else>
            <span>{{ field.label }}{{ field.required ? ' *' : '' }}</span>
            <input
              :value="stringConfigValue(field.key)"
              type="text"
              maxlength="120"
              :pattern="inputPattern(field)"
              autocomplete="off"
              :disabled="!canManage"
              @input="setTextConfig(field.key, $event)"
            />
          </label>

          <small v-if="errorFor(`config.${field.key}`)" class="source-form__error">
            {{ errorFor(`config.${field.key}`) }}
          </small>
        </div>
      </fieldset>

      <fieldset class="source-form__section">
        <legend>Retencao das observacoes</legend>
        <div class="source-form__grid">
          <label>
            <span>Chave da retention policy</span>
            <input
              v-model.trim="draft.retentionPolicyKey"
              type="text"
              maxlength="160"
              pattern="[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*"
              autocomplete="off"
              :disabled="!canManage"
            />
            <small>Deve corresponder a uma versao publicada com o mesmo prazo e acao.</small>
            <small v-if="errorFor('retentionPolicyKey')" class="source-form__error">
              {{ errorFor('retentionPolicyKey') }}
            </small>
          </label>
          <label>
            <span>Prazo do snapshot (dias)</span>
            <input
              v-model.number="retentionDays"
              type="number"
              min="1"
              max="3650"
              step="1"
              :disabled="!canManage"
            />
          </label>
          <AppSelectField
            v-model="draft.onExpiry"
            label="Acao ao expirar"
            :options="[
              { value: 'tombstone', label: 'Tombstone' },
              { value: 'crypto_shred', label: 'Crypto-shred quando cifrado' },
            ]"
            :disabled="!canManage"
          />
        </div>
        <dl v-if="source">
          <div>
            <dt>Versao publicada vinculada</dt>
            <dd>{{ source.retentionPolicyVersion || '-' }}</dd>
          </div>
          <div>
            <dt>Revisao da fonte</dt>
            <dd>{{ source.revision }}</dd>
          </div>
        </dl>
        <p>A policy e versionada no banco. Prompts e conectores nao podem ampliar este prazo.</p>
      </fieldset>

      <p
        v-if="attempted && firstError"
        class="source-form__error source-form__error--summary"
        role="alert"
      >
        {{ firstError }}
      </p>
    </form>

    <template #footer>
      <button
        v-if="descriptor"
        class="source-form__save"
        type="button"
        :disabled="!canManage || saving"
        @click="submit"
      >
        {{ saving ? 'Salvando...' : source ? 'Salvar configuracao' : 'Criar fonte' }}
      </button>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped src="./intelligence-source-config-drawer.css"></style>
