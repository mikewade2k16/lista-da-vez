<script setup>
import { computed, reactive, ref, watch } from 'vue'

const props = defineProps({
  consultants: {
    type: Array,
    default: () => [],
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})

const consultantsSummary = computed(() => {
  const count = props.consultants?.length || 0
  return `${count} ${count === 1 ? 'consultor' : 'consultores'}`
})

const emit = defineEmits(['add', 'update', 'archive'])
const drafts = ref({})
const newConsultant = reactive({
  name: '',
  role: '',
  monthlyGoal: '',
  commissionRate: '',
  conversionGoal: '',
  avgTicketGoal: '',
  paGoal: '',
  color: '#168aad',
})

watch(
  () => props.consultants,
  (consultants) => {
    drafts.value = Object.fromEntries(
      (consultants || []).map((consultant) => [
        consultant.id,
        {
          name: consultant.name,
          role: consultant.role,
          color: consultant.color,
          monthlyGoal: Number(consultant.monthlyGoal || 0),
          commissionRate: Number(consultant.commissionRate || 0),
          conversionGoal: Number(consultant.conversionGoal || 0),
          avgTicketGoal: Number(consultant.avgTicketGoal || 0),
          paGoal: Number(consultant.paGoal || 0),
        },
      ]),
    )
  },
  { immediate: true, deep: true },
)

function submitAdd() {
  emit('add', { ...newConsultant })
  newConsultant.name = ''
  newConsultant.role = ''
  newConsultant.monthlyGoal = ''
  newConsultant.commissionRate = ''
  newConsultant.conversionGoal = ''
  newConsultant.avgTicketGoal = ''
  newConsultant.paGoal = ''
  newConsultant.color = '#168aad'
}
</script>

<template>
  <article class="settings-card">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Gestao de consultores</h3>
      <p class="settings-card__text">
        CRUD administrativo de perfil, meta, comissao e acesso vinculado.
      </p>
      <p class="settings-card__text">
        Cada consultor passa a nascer com login automatico e email padrao por loja.
      </p>
    </header>

    <details class="settings-collapse">
      <summary class="settings-collapse__summary">
        <div class="settings-collapse__title-wrap">
          <strong class="settings-collapse__title">Consultores cadastrados</strong>
          <span class="settings-collapse__text">Editar perfil, meta, comissao e acesso</span>
        </div>
        <span class="settings-collapse__meta">{{ consultantsSummary }}</span>
        <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
          expand_more
        </span>
      </summary>

      <div class="settings-collapse__body">
        <div class="consultant-head">
          <span>Nome</span>
          <span>Login</span>
          <span>Cargo</span>
          <span>Cor</span>
          <span>Meta R$</span>
          <span>Comissao</span>
          <span>Conv. alvo %</span>
          <span>Ticket alvo</span>
          <span>P.A. alvo</span>
          <span></span>
          <span></span>
        </div>

        <div class="option-list">
          <span v-if="!consultants.length" class="insight-empty">Nenhum consultor cadastrado.</span>
          <form
            v-for="consultant in consultants"
            :key="consultant.id"
            class="consultant-row"
            @submit.prevent="$emit('update', consultant.id, drafts[consultant.id])"
          >
            <input
              v-if="drafts[consultant.id]"
              v-model="drafts[consultant.id].name"
              class="product-row__input"
              type="text"
              :disabled="disabled"
            />
            <span class="settings-card__text">
              {{ consultant.access?.email || 'Acesso pendente' }}
            </span>
            <input
              v-if="drafts[consultant.id]"
              v-model="drafts[consultant.id].role"
              class="product-row__input"
              type="text"
              :disabled="disabled"
            />
            <input
              v-if="drafts[consultant.id]"
              v-model="drafts[consultant.id].color"
              class="product-row__input"
              type="color"
              :disabled="disabled"
            />
            <input
              v-if="drafts[consultant.id]"
              v-model="drafts[consultant.id].monthlyGoal"
              class="product-row__input"
              type="number"
              min="0"
              step="100"
              :disabled="disabled"
            />
            <input
              v-if="drafts[consultant.id]"
              v-model="drafts[consultant.id].commissionRate"
              class="product-row__input"
              type="number"
              min="0"
              max="1"
              step="0.001"
              :disabled="disabled"
            />
            <input
              v-if="drafts[consultant.id]"
              v-model="drafts[consultant.id].conversionGoal"
              class="product-row__input"
              type="number"
              min="0"
              max="100"
              step="1"
              placeholder="0"
              :disabled="disabled"
            />
            <input
              v-if="drafts[consultant.id]"
              v-model="drafts[consultant.id].avgTicketGoal"
              class="product-row__input"
              type="number"
              min="0"
              step="100"
              placeholder="0"
              :disabled="disabled"
            />
            <input
              v-if="drafts[consultant.id]"
              v-model="drafts[consultant.id].paGoal"
              class="product-row__input"
              type="number"
              min="0"
              step="0.1"
              placeholder="0"
              :disabled="disabled"
            />
            <button class="option-row__save" type="submit" :disabled="disabled">Salvar</button>
            <button
              class="product-row__remove"
              type="button"
              :disabled="disabled"
              @click="$emit('archive', consultant.id)"
            >
              Arquivar
            </button>
          </form>
        </div>
      </div>
    </details>

    <details class="settings-collapse">
      <summary class="settings-collapse__summary">
        <div class="settings-collapse__title-wrap">
          <strong class="settings-collapse__title">Adicionar consultor</strong>
          <span class="settings-collapse__text">Novo perfil com login e email automaticos</span>
        </div>
        <span class="settings-collapse__meta">Novo</span>
        <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
          expand_more
        </span>
      </summary>

      <div class="settings-collapse__body">
        <form class="consultant-add" @submit.prevent="submitAdd">
          <input
            v-model="newConsultant.name"
            class="product-add__input"
            type="text"
            placeholder="Nome do consultor"
            :disabled="disabled"
          />
          <span class="settings-card__text">Email gerado automaticamente</span>
          <input
            v-model="newConsultant.role"
            class="product-add__input"
            type="text"
            placeholder="Cargo (ex: Atendimento)"
            :disabled="disabled"
          />
          <input
            v-model="newConsultant.color"
            class="product-add__input"
            type="color"
            :disabled="disabled"
          />
          <input
            v-model="newConsultant.monthlyGoal"
            class="product-add__input"
            type="number"
            min="0"
            step="100"
            placeholder="Meta R$"
            :disabled="disabled"
          />
          <input
            v-model="newConsultant.commissionRate"
            class="product-add__input"
            type="number"
            min="0"
            max="1"
            step="0.001"
            placeholder="Comissao (0.03)"
            :disabled="disabled"
          />
          <input
            v-model="newConsultant.conversionGoal"
            class="product-add__input"
            type="number"
            min="0"
            max="100"
            step="1"
            placeholder="Conv. alvo %"
            :disabled="disabled"
          />
          <input
            v-model="newConsultant.avgTicketGoal"
            class="product-add__input"
            type="number"
            min="0"
            step="100"
            placeholder="Ticket alvo R$"
            :disabled="disabled"
          />
          <input
            v-model="newConsultant.paGoal"
            class="product-add__input"
            type="number"
            min="0"
            step="0.1"
            placeholder="P.A. alvo"
            :disabled="disabled"
          />
          <button class="product-add__button" type="submit" :disabled="disabled">
            Adicionar consultor
          </button>
        </form>
      </div>
    </details>
  </article>
</template>
