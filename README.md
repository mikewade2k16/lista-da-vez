# Fila de Atendimento MVP

Base inicial em HTML + JavaScript modular para evoluir rapido e migrar depois para Nuxt sem reescrever regra de negocio.

## Estrutura

- `index.html`: ponto de entrada do MVP.
- `src/main.js`: bootstrap, render e eventos globais.
- `src/pages`: telas compostas.
- `src/components`: blocos reutilizaveis de interface.
- `src/services`: integracoes e camada de dados.
- `src/store`: estado central da aplicacao.
- `src/data`: mocks locais para acelerar validacao.
- `src/utils`: funcoes auxiliares puras.
- `src/styles`: tokens, base, layout e componentes.

## Convencao para futura migracao para Nuxt

- Componentes atuais podem virar `components/`.
- Paginas atuais podem virar `pages/`.
- Service continua como camada de API.
- Store atual pode migrar para composables ou Pinia.
- CSS ja esta separado por responsabilidade.

## Como evoluir

1. Ajustar layout e fluxo da lista da vez.
2. Substituir mocks por API real.
3. Migrar a composicao para Nuxt quando o fluxo estiver validado.

## Areas atuais no MVP

- `Operacao`: fila, atendimento, pausa e fechamento.
- `Consultor`: meta mensal, progresso, indicadores e simulador.
- `Ranking`: comparativo mensal e diario entre consultores.
- `Dados`: painel bruto de produto, motivo, origem, horario e tempo.
- `Inteligencia`: leitura automatica dos dados com diagnostico e acoes recomendadas.
- `Relatorios`: filtros avancados com exportacao CSV/PDF.
- `Campanhas`: regras comerciais aplicadas no fechamento com auditoria de bonus.
- `Multi-loja`: operacao por loja + visao consolidada comparativa.
- `Configuracoes`: administra campos/opcoes do modal, modo teste e catalogo.
- `Perfis`: troca rapida por dropdown no header (`admin`, `manager`, `consultant`).

## Status e backlog

- Backlog oficial e historico de entregas: `docs/BACKLOG.md`
- Documento tecnico completo para migracao Nuxt: `docs/NUXT_MIGRATION_BLUEPRINT.md`
- Data da ultima organizacao do backlog: `2026-03-13`

## CRUD de consultores

- CRUD administrativo de consultores ativo na area `Configuracoes`.
- Contrato tecnico de repositorio mantido em:
- `src/services/consultant-admin-repository.js`
- Status atual do P1: `parcial` (sem backend/login por enquanto).
- Status atual do P2: `parcial` (sem API real de produtos; relatorios e campanhas concluidos em local).
- Status atual do P3: `concluido` (multi-loja com visao consolidada).

## Execucao local

- `npm run dev`
- Abrir `http://127.0.0.1:4173`

## Perfis de teste no header

- `Admin Nexo` (acesso total)
- `Gerente Loja` (operacao, dados e leitura)
- `Consultor Loja` (operacao e paineis permitidos)
