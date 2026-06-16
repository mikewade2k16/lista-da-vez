# AGENT - Settings Components

## Escopo

Componentes administrativos de configuracoes em `web/app/components/settings/`.

## Padrao Atual

- `SettingsWorkspace.vue` e o host das abas.
- Secoes novas ficam em `web/app/components/settings/sections/`.
- `useSettingsWorkspace.js` concentra store actions, permissao, validacoes e helpers do modal.
- `settings-workspace-data.js` concentra tabs, configuracoes estaticas e listas de campos.
- `SettingsOptionManager.vue`, `SettingsProductManager.vue` e `SettingsConsultantManager.vue` seguem como componentes especializados.
- `sections/SettingsCrmGoalsSection.vue` edita a politica comercial do CRM: faixas de uso da lista, pedidos minimos para destaque e recebimento por atingimento de meta (grupos Consultor/Gerente/Caixa e auxiliar).
  - As faixas de recebimento usam **rascunho local por grupo** com `_id` estavel: o usuario digita livremente (valor 0/vazio transitorio NAO derruba a linha) e a persistencia so acontece em acao explicita (`blur` do input, `change` do select de tipo, botao "Salvar faixas" e na remocao) — nunca a cada tecla. A ordenacao por threshold so ocorre no save. `:key` e o `_id` local (nunca `rule.threshold`), evitando perda de foco/reordenacao no meio da digitacao. Validacao amigavel (threshold/valor invalido, duplicado) via `ui.error`. Grupos sao accordion (`<details>/<summary>` + classes `settings-collapse*`) com resumo de nº de faixas. Remover desce ate zero faixas (persiste array vazio — depende de `normalizeCrmGoalPayoutPolicy` preservar `[]`, ver `crm-performance-policy.ts`). Save via nova `saveCrmGoalPayoutGroup` em `useSettingsWorkspace.js`.
- A persistencia da politica comercial usa `settingsStore.updateCrmCommercialPolicy()` enviando apenas esses campos em `settings` para `PATCH /v1/settings/operation`, mantendo compatibilidade com backends que ainda nao registraram o endpoint dedicado.
- `sections/SettingsScoreWeightsCard.vue` exibe os 5 sliders de pesos do Score 360 (Conversao/Valor vendido/Qualidade/P.A./Disciplina de fila) na aba Gamificacao. Persiste via `ctx.updateNumericSetting` (mesmo caminho da aba Operacao: `PATCH /v1/settings/operation` com `settings.scoreWeight*`). Mostra total em tempo real; aviso visual quando soma != 100.

## Regras Locais

- Nao chamar stores diretamente dentro de componentes de secao.
- Manter cada secao abaixo de 500 linhas e preferir props simples via `ctx`.
- Preservar contratos de `settingsStore` e `consultantsStore`; mudancas de persistencia devem ficar no store.
- `sections/SettingsCrmGoalsSection.vue` so deve habilitar edicao quando `ctx.canEditCrmCommercialPolicy` for verdadeiro (`platform_admin` ou `director`).
- Para validar alteracoes, abrir `/configuracoes`, alternar abas e testar uma alteracao reversivel.
