# AC-07b · Onda 2c — Refactor dos stores centrais (6 stores, ~4,8k linhas)

> Spec de implementação · Prioridade **P1** · Esforço **L** · Impacto **médio**
> Origem: AC-07 recorte 2 · roadmap `ac-fixes-2026-07` → task `ac-07b-refactor-front-recorte-2`
> Censo 03/07 (re-medir): `app/stores/erp.ts` **1.566** · `settings.ts` **732** · `auth.ts` **683** ·
> `alerts.ts` **662** · `operations.ts` **650** · `multistore.ts` **460**

## 1. Contexto

Os stores centrais do app cresceram além do teto. `erp.ts` era backlog explícito do AC-07 ("fica
para o recorte 2"); `auth.ts` é a fonte de verdade da sessão (TODO cuidado — ver regras);
`operations.ts` alimenta a página mais crítica do produto. Molde pronto:
`app/stores/dashboard/runtime/` (`create-dashboard-runtime.ts` + `state.ts` + `actions/*-actions.ts`).

## 2. Objetivo e não-objetivos

**Objetivo:** cada store vira casca ≤300 (`defineStore(id, ...)` com o MESMO id, montando slices) +
diretório `app/stores/<nome>/` com state/slices ≤450 cada. API pública 100% idêntica.

**Não-objetivos (FORA):** `app/stores/dashboard.ts` + `dashboard/runtime/**` (o `state.ts` de 841
vai na onda 2f — o motor runtime é mais arriscado e merece lote próprio); `cardapio.ts` (2f);
`calendar.ts` (INTOCÁVEL); qualquer mudança de comportamento, persistência ou naming.

## 3. Regras de execução (obrigatórias)

Bloco padrão (sem git; re-medir/re-ler; casca+barril; ≤450; type-check+vitest via container;
AGENT.md). Extras CRÍTICOS desta onda:
- **IDs de store IMUTÁVEIS** (`defineStore('auth', ...)` etc.) — realtime, persist e devtools dependem.
- **`auth.ts`:** NÃO tocar na semântica de sessão/expiração (token 12h; `auth-bridge.client.ts`
  depende do shape), NEM em `ldv_remembered_login` (localStorage é decisão de produto — manter).
  Fatiar SÓ movendo código; um teste manual de login/logout/refresh é obrigatório no smoke.
- **`operations.ts`:** o realtime da operação (`useOperationsRealtime`) consome este store — o
  teste `operations.test.ts` e `useOperationsRealtime.test.ts` verdes SEM edição são o gate.
- **Divergência de fonte de conta (memória):** `accountStore.activeAccountId` (v2) vs
  `auth.activeTenantId` — NÃO "unificar" nada durante o refactor; mover como está.
- Ordem interna recomendada: erp → settings → alerts → multistore → operations → auth (do menos
  para o mais sensível; validar type-check entre cada um).

## 4. Mudanças (passo a passo — repetir o mesmo procedimento por store)

Para cada store `<nome>` (erp, settings, auth, alerts, operations, multistore):

1. RE-LER o arquivo; mapear blocos (state, getters, actions por domínio).
2. Criar `app/stores/<nome>/`:
   - `state.ts` — refs/tipos do estado (exportar `createXxxState()`);
   - `<dominio>-slice.ts` — `createXxxSlice(deps)` por grupo de actions/getters coesos
     (ex.: erp → `sync-slice`, `items-slice`, `filters-slice`, `export-slice`;
     settings → `load-slice`, `save-slice`; auth → `session-slice`, `context-slice`,
     `permissions-slice`; alerts → `list-slice`, `actions-slice`; operations →
     `snapshot-slice`, `commands-slice`; multistore → `stores-slice`, `overview-slice` —
     ajustar aos blocos reais da leitura);
   - deps passadas por parâmetro (api-client, outros stores via `useXxxStore()` DENTRO do slice
     factory quando já era assim).
3. `app/stores/<nome>.ts` vira casca: `defineStore('<id original>', () => { const state = ...;
   const a = createASlice({...}); ... return { ...state, ...a, ...b } })` — TODOS os nomes
   retornados idênticos aos atuais (conferir por diff da lista de keys).
4. Rodar `vue-tsc --noEmit` antes de passar ao próximo store.

**Detector de quebra:** os testes existentes `auth.test.ts`, `settings.test.ts`,
`multistore.test.ts`, `operations.test.ts` passam SEM QUALQUER EDIÇÃO. Teste que falhar = contrato
quebrado = corrigir o refactor.

## 5. Critérios de aceite

1. 6 cascas ≤300 e todos os novos ≤450 (`wc -l`).
2. `vue-tsc --noEmit` limpo; `vitest run` verde sem editar testes.
3. Ids de store inalterados (`grep -rn "defineStore(" app/stores/*.ts` — mesmos literais).
4. Smoke com o dono: login/logout, troca de conta ativa, /operacao com realtime (WS) atualizando,
   /erp sincronizando lista, configurações salvando, alertas listando.
5. Nenhum consumidor mudou import (`grep -rn "stores/erp'\|stores/auth'\|stores/settings'" web/` — os
   paths originais resolvem).

## 6. Validação

```bash
docker compose run --rm web npx vue-tsc --noEmit
docker compose run --rm web npx vitest run
npm run dev:watch   # smoke do item 4 do aceite (dono)
```

## 7. Notas de Deploy

Nenhuma migration/env. Rebuild web no próximo deploy. Rollback: reverter arquivos.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `app/stores/{erp,settings,auth,alerts,operations,multistore}.ts` | editar (cascas) |
| `app/stores/{erp,settings,auth,alerts,operations,multistore}/*.ts` | criar (state+slices) |
| `web/AGENT.md` | editar |

**Conflitos potenciais:** AC-15b escreve TESTES para vários destes stores — executar 2c ANTES do
AC-15b (os testes novos já nascem contra as cascas). Onda 2b importa `operations/multistore` —
sequencial 2b → 2c recomendado.
