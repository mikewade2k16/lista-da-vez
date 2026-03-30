# Fila de Atendimento MVP

Repositorio dividido entre `web/` para o Nuxt, `core/` para regras compartilhadas em TypeScript e `back/` para o backend em Go.

## Estrutura

- `web/`: app Nuxt 4, stores Pinia, componentes Vue, assets de estilo e servicos de browser.
- `core/`: dominio, mocks, regras, calculos e utilitarios compartilhados em TypeScript.
- `back/`: bootstrap do backend em Go para API, auth, websocket e integracao com banco.
- `docs/`: backlog, blueprint de migracao e referencia funcional.

## Estado atual da migracao

- Todas as areas do painel ja estao em paginas e componentes Nuxt dentro de `web/`.
- O estado de interface roda em `Pinia`.
- Os estilos globais do frontend vivem em `web/app/assets/styles/`.
- O dominio temporario local foi isolado em `core/`.
- O backend ainda esta em bootstrap, pronto para receber a API em Go.

## Como evoluir

1. Substituir `localStorage` por API real.
2. Introduzir autenticacao, multi-dispositivo e sincronizacao em tempo real.
3. Portar regras autoritativas de fila/atendimento/campanhas para o `back/`.
4. Extrair o modulo da lista da vez para reutilizacao no stack `Nuxt + Go`.

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
- Referencia consolidada do Nuxt 4 usada na migracao: `docs/NUXT_FULL_REFERENCE.md`
- Data da ultima organizacao do backlog: `2026-03-13`

## Execucao local

- Pela raiz:
- `npm install --prefix web`
- `npm run dev`
- Abrir `http://localhost:3000`
- Se quiser manter em `3001`, usar `npm run dev:3001`
- Direto no frontend:
- `cd web`
- `npm install`
- `npm run dev`
- ou `npm run dev:3001`
- Backend:
- `cd back`
- `go run ./cmd/api`
- Healthcheck: `http://localhost:8080/healthz`

## Modos disponiveis

- Pela raiz:
- `npm run dev`: sobe o app Nuxt 4 em `web/`.
- `npm run dev:3001`: sobe o app Nuxt 4 em `http://localhost:3001`.
- `npm run build`: roda o build do frontend.
- `npm run generate`: gera a saida estatico/prerender do frontend.
- Em `web/`:
- `npm run dev`: sobe o app Nuxt 4.
- `npm run dev:3001`: sobe o app Nuxt 4 em `3001`.
- `npm run build`: build SSR padrao do Nuxt.
- `npm run generate`: gera saida estatico/prerender do Nuxt.
- Em `back/`:
- `go run ./cmd/api`: sobe o bootstrap inicial da API Go.

## Perfis de teste no header

- `Admin Nexo` (acesso total)
- `Gerente Loja` (operacao, dados e leitura)
- `Consultor Loja` (operacao e paineis permitidos)
