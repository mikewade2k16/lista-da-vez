# AGENT - ERP Components

## Escopo

Componentes administrativos da workspace ERP em `web/app/components/erp/`.

## Padrao Atual

- `ErpWorkspace.vue` e o host fino da tela: header, tabs e composicao das abas.
- `ErpProductsTab.vue`, `ErpRecordsTab.vue`, `ErpBancoTab.vue` e `ErpSyncTab.vue` concentram a UI das areas principais.
- `ErpCrmWorkspace.vue`, `ErpDataTable.vue`, `ErpSyncOverview.vue`, `ErpSyncRunsTable.vue`, `ErpSyncRunDetail.vue` e `ErpSyncStatus.vue` continuam como componentes especializados reutilizados.
- Constantes, colunas e formatadores puros ficam em `web/app/domain/utils/erp-display.ts`.
- Orquestracao de store, filtros e watchers fica em `web/app/composables/useErpWorkspace.ts`.
- Exportacao CSV, acoes de sync/bootstrap e notificacoes automaticas ficam em composables dedicados.

## Regras Locais

- Nao mover regra de store para componentes visuais.
- Manter abas com props/emits explicitos e arquivos abaixo de 500 linhas.
- Reaproveitar `ErpDataTable.vue` para grades administrativas em vez de criar tabelas paralelas.
- Antes de alterar filtros, paginacao ou exportacao, validar `/erp` manualmente com troca de aba.
