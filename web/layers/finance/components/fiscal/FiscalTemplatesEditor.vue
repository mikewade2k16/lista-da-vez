<script setup lang="ts">
import { computed } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { confirmFiscalAction } from '../../utils/fiscal-confirm'
import type { FiscalProfileEditor } from '../../composables/useFiscalProfile'
import type { FiscalTemplate } from '../../types/fiscal'
const props = defineProps<{ editor: FiscalProfileEditor }>()
const { templates, templateDraft, templateDirty, editingId, previewId, preview, view } =
  props.editor
const withheldOptions = [
  { value: '', label: 'Não informado' },
  { value: 'yes', label: 'Sim' },
  { value: 'no', label: 'Não' },
]
const withheldModel = computed({
  get: () =>
    templateDraft.fields.issWithheld === null
      ? ''
      : templateDraft.fields.issWithheld
        ? 'yes'
        : 'no',
  set: (value: string) => {
    templateDraft.fields.issWithheld = value === 'yes' ? true : value === 'no' ? false : null
    templateDirty.value = true
  },
})
const previewOptions = computed(() =>
  templates.value.map((item) => ({ value: item.id, label: item.name })),
)
function setISSRate(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  templateDraft.fields.issRate = raw === '' ? null : Number(raw)
}
function mayDiscard() {
  return (
    !templateDirty.value || window.confirm('Descartar as alterações do template ainda não salvas?')
  )
}
async function edit(template: FiscalTemplate) {
  if (await mayDiscard()) props.editor.editTemplate(template)
}
async function clear() {
  if (await mayDiscard()) props.editor.clearTemplate()
}
async function remove(template: FiscalTemplate) {
  if (
    await confirmFiscalAction(
      `Excluir o template "${template.name}" deste perfil?`,
      () => props.editor.scopeKey.value,
    )
  )
    await props.editor.deleteTemplate(template)
}
</script>
<template>
  <details class="fiscal-profile__section">
    <summary>
      Templates de serviço
      <span>{{ templates.length }} cadastrados</span>
    </summary>
    <p class="fiscal-profile__hint">
      Salve os campos que se repetem. Dados do emitente vêm do perfil; valores e tomador serão
      definidos na preparação de cada nota.
    </p>
    <ul v-if="templates.length" class="fiscal-profile__templates">
      <li v-for="item in templates" :key="item.id">
        <span>
          {{ item.name }}
          <strong v-if="item.isDefault">(padrão)</strong>
        </span>
        <div class="fiscal-profile__actions">
          <AppPanelButton variant="secondary" @click="edit(item)">Editar</AppPanelButton>
          <AppPanelButton variant="secondary" @click="remove(item)">Excluir</AppPanelButton>
        </div>
      </li>
    </ul>
    <p v-else class="fiscal-profile__hint">Nenhum template cadastrado. Crie o primeiro abaixo.</p>
    <form
      class="fiscal-profile__form"
      @submit.prevent="editor.saveTemplate"
      @input="templateDirty = true"
      @change="templateDirty = true"
    >
      <h4>{{ editingId ? 'Editar template' : 'Novo template' }}</h4>
      <div class="fiscal-profile__grid">
        <label class="fiscal-profile__field">
          <span>Nome do template</span>
          <input v-model="templateDraft.name" maxlength="100" required />
        </label>
        <label class="fiscal-profile__check">
          <input v-model="templateDraft.isDefault" type="checkbox" />
          <span>Usar como padrão neste perfil</span>
        </label>
        <label class="fiscal-profile__field fiscal-profile__field--wide">
          <span>Descrição do serviço</span>
          <textarea
            v-model="templateDraft.fields.serviceDescription"
            maxlength="4000"
            rows="3"
            required
          ></textarea>
        </label>
        <label class="fiscal-profile__field">
          <span>Código de serviço (opcional)</span>
          <input v-model="templateDraft.fields.serviceCode" maxlength="60" />
        </label>
        <label class="fiscal-profile__field">
          <span>CNAE (opcional)</span>
          <input v-model="templateDraft.fields.cnae" maxlength="60" />
        </label>
        <label class="fiscal-profile__field">
          <span>NBS (opcional)</span>
          <input v-model="templateDraft.fields.nbs" maxlength="60" />
        </label>
        <label class="fiscal-profile__field">
          <span>ISS em % (opcional)</span>
          <input
            :value="templateDraft.fields.issRate ?? ''"
            type="number"
            min="0"
            max="100"
            step="0.01"
            placeholder="Não informado"
            @input="setISSRate"
          />
        </label>
        <AppSelectField
          v-model="withheldModel"
          label="ISS retido"
          :options="withheldOptions"
          :disabled="editor.loading.value || editor.saving.value"
        />
        <label class="fiscal-profile__field fiscal-profile__field--wide">
          <span>Observações padrão (opcional)</span>
          <textarea v-model="templateDraft.fields.notes" rows="2" maxlength="2000"></textarea>
        </label>
      </div>
      <div class="fiscal-profile__actions">
        <AppPanelButton type="submit">Salvar template</AppPanelButton>
        <AppPanelButton variant="secondary" @click="clear">
          {{ editingId ? 'Cancelar edição' : 'Limpar formulário' }}
        </AppPanelButton>
        <span v-if="templateDirty" role="status">Alterações não salvas</span>
      </div>
    </form>
    <div v-if="templates.length" class="fiscal-profile__preview">
      <AppSelectField
        v-model="previewId"
        label="Prévia de preenchimento"
        :options="previewOptions"
        :disabled="editor.loading.value || editor.saving.value"
      />
      <dl v-if="preview" class="fiscal-profile__preview-fields">
        <div>
          <dt>Emitente do perfil</dt>
          <dd>{{ view?.profile.legalName || 'Não informado' }}</dd>
        </div>
        <div>
          <dt>Município / UF</dt>
          <dd>
            {{ view?.profile.municipality || 'Não informado' }} /
            {{ view?.profile.state || 'Não informada' }}
          </dd>
        </div>
        <div>
          <dt>Descrição</dt>
          <dd>{{ preview.fields.serviceDescription }}</dd>
        </div>
        <div>
          <dt>Código do serviço / CNAE / NBS</dt>
          <dd>
            {{ preview.fields.serviceCode || 'Não informado' }} /
            {{ preview.fields.cnae || 'Não informado' }} /
            {{ preview.fields.nbs || 'Não informado' }}
          </dd>
        </div>
        <div>
          <dt>ISS / Retenção</dt>
          <dd>
            {{ preview.fields.issRate === null ? 'Não informado' : `${preview.fields.issRate}%` }} /
            {{
              preview.fields.issWithheld === null
                ? 'Não informada'
                : preview.fields.issWithheld
                  ? 'Sim'
                  : 'Não'
            }}
          </dd>
        </div>
        <div>
          <dt>Observações</dt>
          <dd>{{ preview.fields.notes || 'Não informadas' }}</dd>
        </div>
      </dl>
      <p class="fiscal-profile__notice">
        Prévia de cadastro. Não é uma nota emitida e não envia dados ao portal.
      </p>
    </div>
  </details>
</template>
