# AC-07b · Onda 2e — Refactor composables/domain + OmniEditor (~4,1k linhas)

> Spec de implementação · Prioridade **P1** · Esforço **L** · Impacto **médio**
> Origem: AC-07 recorte 2 · roadmap `ac-fixes-2026-07` → task `ac-07b-refactor-front-recorte-2`
> Censo 03/07 (re-medir): `app/components/omni/OmniEditor.vue` **948** ·
> `app/domain/utils/permissions.ts` **843** · `app/domain/utils/admin-metrics.ts` **715** ·
> `app/domain/utils/reports.ts` **633** · `app/composables/useUsersAccessManager.*` (medir — entrou
> no censo geral; se ≤450, remover da onda)
> Opcional (SÓ com liberação explícita do dono): `layers/finance/composables/useFinanceSheetEditor.ts` **905**

## 1. Contexto

Os utilitários de domínio (`permissions`, `admin-metrics`, `reports`) são os arquivos mais
IMPORTADOS do app — tamanho deles pesa em todo type-check e todo contexto de agente. São também os
mais fáceis de fatiar: funções puras, com testes existentes (`permissions.test.ts`,
`permissions-workspaces.test.ts`, `reports.test.ts`) que servem de rede de segurança. O
`OmniEditor.vue` (TipTap) é o editor compartilhado — tem regras delicadas de bundling
(registro de falhas nº 10: NUNCA marcar dep TipTap como external; extensões pré-bundladas no
`nuxt.config.ts` `optimizeDeps`).

## 2. Objetivo e não-objetivos

**Objetivo:** 4 (ou 5) arquivos ≤450 via split por responsabilidade com BARRIL no path original
(re-export — imports dos consumidores intocados).
**Não-objetivos (FORA):** mudar QUALQUER regra de permissão (é segurança — split é só mover);
layer finance INTEIRA a menos que o dono libere por escrito na hora do despacho; `nuxt.config.ts`.

## 3. Regras de execução

Bloco padrão da rodada. Extras:
- **`permissions.ts` é código de SEGURANÇA:** split mecânico, função por função, SEM reescrever
  lógica; os 2 arquivos de teste existentes passam sem edição. Lembrar a regra do gating
  (`isPlatformAdmin || has(...)`) — não "consertar" nada no caminho.
- **Dado declarativo → barril** (padrão roadmap-data): se um bloco for tabela/constante grande
  (mapas de workspace, defaults de métricas), vira arquivo de dado puro.
- OmniEditor: NÃO tocar na lista de extensões TipTap nem em imports dinâmicos; só extrair UI/lógica.

## 4. Mudanças (passo a passo)

### 4.1 `app/domain/utils/permissions.ts` (843 → barril ≤80)
Criar `app/domain/utils/permissions/` com split esperado (ajustar à leitura): `types.ts`,
`matrix.ts` (dados/mapas por papel/workspace), `checks.ts` (has/can*), `workspaces.ts`
(acesso por workspace), `admin.ts` (platform_admin/coarse legado). `permissions.ts` reexporta TUDO
(`export * from './permissions/checks'` etc.). Testes existentes verdes SEM edição.

### 4.2 `app/domain/utils/admin-metrics.ts` (715 → barril)
Split em `admin-metrics/` por métrica/fonte (ex.: `aggregations.ts`, `formatting.ts`, `types.ts`).

### 4.3 `app/domain/utils/reports.ts` (633 → barril)
Split em `reports/` (períodos/janelas, agregação, formatação). `reports.test.ts` verde sem edição.

### 4.4 `app/components/omni/OmniEditor.vue` (948 → ≤450)
Extrair `app/components/omni/editor/`: `useOmniEditorSetup.ts` (criação do editor, extensões —
mover o bloco de extensões COMO ESTÁ), `EditorToolbar.vue`, `EditorBubbleMenu.vue` (conforme
template). Casca mantém props/emits/v-model. **Smoke obrigatório: modal de task abre com editor
funcionando em dev E num build de prod local (`npm run prod:up`) — o registro de falhas nº 10
aconteceu exatamente aqui e só aparecia em prod.**

### 4.5 `useUsersAccessManager` — medir primeiro
`wc -l` no arquivo real; >450 → split em sub-composables (`users-access/`); ≤450 → registrar na
spec como "fora, já dentro do teto".

## 5. Critérios de aceite

1. Cascas/barris + novos ≤450; barris SÓ reexportam (zero lógica).
2. `vue-tsc --noEmit` + `vitest run` verdes SEM editar os testes existentes de permissions/reports.
3. Grep de consumidores: nenhum import mudou (`~/domain/utils/permissions` continua resolvendo).
4. Smoke do editor em dev + build prod local (item 4.4) com o dono.
5. Diff de comportamento de permissão = ZERO (mesmos resultados de `has()`/gates para os papéis de
   teste — rodar os 2 test files é o gate).

## 6. Validação

```bash
docker compose run --rm web npx vue-tsc --noEmit
docker compose run --rm web npx vitest run
npm run prod:up   # smoke do editor em build de prod local (depois prod:down)
```

## 7. Notas de Deploy
Nenhuma migration/env; rebuild web; rollback = reverter arquivos.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `app/domain/utils/permissions.ts` → `permissions/*` | editar (barril) + criar |
| `app/domain/utils/admin-metrics.ts` → `admin-metrics/*` | editar (barril) + criar |
| `app/domain/utils/reports.ts` → `reports/*` | editar (barril) + criar |
| `app/components/omni/OmniEditor.vue` + `omni/editor/*` | editar + criar |
| `app/composables/useUsersAccessManager.*` (se >450) | editar + criar |
| `web/AGENT.md` | editar |

**Conflitos potenciais:** quase todo o app importa `permissions`/`reports` — rodar esta onda SOZINHA
(sem outra onda em paralelo) e com type-check entre cada split.
