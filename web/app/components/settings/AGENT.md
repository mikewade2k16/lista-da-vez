# AGENT - Settings Components

## Escopo

Componentes administrativos de configuracoes em `web/app/components/settings/`.

## Padrao Atual

- `SettingsWorkspace.vue` e o host das abas.
- Secoes novas ficam em `web/app/components/settings/sections/`.
- `useSettingsWorkspace.js` concentra store actions, permissao, validacoes e helpers do modal.
- `settings-workspace-data.js` concentra tabs, configuracoes estaticas e listas de campos.
- `SettingsOptionManager.vue`, `SettingsProductManager.vue` e `SettingsConsultantManager.vue` seguem como componentes especializados.
- `sections/SettingsCrmGoalsSection.vue` edita a politica comercial do CRM: faixas de uso da lista, pedidos minimos para destaque e recebimento por atingimento de meta.
- A persistencia da politica comercial usa `settingsStore.updateCrmCommercialPolicy()` enviando apenas esses campos em `settings` para `PATCH /v1/settings/operation`, mantendo compatibilidade com backends que ainda nao registraram o endpoint dedicado.

## Regras Locais

- Nao chamar stores diretamente dentro de componentes de secao.
- Manter cada secao abaixo de 500 linhas e preferir props simples via `ctx`.
- Preservar contratos de `settingsStore` e `consultantsStore`; mudancas de persistencia devem ficar no store.
- `sections/SettingsCrmGoalsSection.vue` so deve habilitar edicao quando `ctx.canEditCrmCommercialPolicy` for verdadeiro (`platform_admin` ou `director`).
- Para validar alteracoes, abrir `/configuracoes`, alternar abas e testar uma alteracao reversivel.
