# AGENT - Settings Components

## Escopo

Componentes administrativos de configuracoes em `web/app/components/settings/`.

## Padrao Atual

- `SettingsWorkspace.vue` e o host das abas.
- Secoes novas ficam em `web/app/components/settings/sections/`.
- `useSettingsWorkspace.js` concentra store actions, permissao, validacoes e helpers do modal.
- `settings-workspace-data.js` concentra tabs, configuracoes estaticas e listas de campos.
- `SettingsOptionManager.vue`, `SettingsProductManager.vue` e `SettingsConsultantManager.vue` seguem como componentes especializados.

## Regras Locais

- Nao chamar stores diretamente dentro de componentes de secao.
- Manter cada secao abaixo de 500 linhas e preferir props simples via `ctx`.
- Preservar contratos de `settingsStore` e `consultantsStore`; mudancas de persistencia devem ficar no store.
- Para validar alteracoes, abrir `/configuracoes`, alternar abas e testar uma alteracao reversivel.
