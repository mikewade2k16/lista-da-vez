# AGENTS — web/app/components/meta-ads

## Escopo

Estas instruções valem para `web/app/components/meta-ads`. Regras herdadas:
`AGENT_RULES.md`, `docs/ENGINEERING_PRINCIPLES.md` e
`docs/meta-ads/ASSISTENTE_360_STATUS_E_ROADMAP.md`.

## Responsabilidade

Esta pasta contém a UI do workspace `meta_ads`: conexão first-party, seleção e
sincronização de contas de anúncio, KPIs/relatórios, campanhas read-only, vínculos
agência→cliente e políticas de ações. Ela não possui chat próprio.

O Assistente 360 é compartilhado com Calendário/global e montado uma única vez em
`OmniAssistantHost`, no layout autenticado. O workspace apenas abre uma nova conversa
com `surface=meta_ads` quando necessário. Conversas, voz, configuração, memória, cards e
confirmações vivem nos componentes/composables compartilhados.

O frontend nunca recebe token Meta, não chama a Graph diretamente e não executa
mutação de campanha. Confirmações de ações chamam o backend durável; o backend é quem
revalida RBAC, conta, cliente, policy, snapshot e idempotência.

## Componentes atuais

- `MetaAdsWorkspace.vue`: orquestra as abas, range, sync e cleanup das requisições;
  abre o Assistente 360 na surface Meta. Sync exige `meta_ads.manage` efetiva.
- `MetaAdsConnectionCard.vue`: Facebook Login como caminho principal e System User
  token dentro de compatibilidade avançada. Login/2FA/consentimento continuam humanos;
  callback e persistência cifrada são server-side. O composable
  `useMetaAdsConnectionContext` prende popup/token/poll à account ativa e limpa tudo
  sincronicamente na troca A→B.
- `MetaAdsClientMappingCard.vue`: para agência, vincula ad account e identidade
  Page/Instagram a um cliente ativo da mesma organização. Sem `meta_ads.manage`, é
  read-only. O backend revalida a identidade na Graph antes do PATCH.
- `MetaAdsActionPolicyCard.vue`: lê a policy efetiva por ad account e, somente para a
  agência com manage, configura gates e tetos diário/lifetime. O gate create também
  controla a promoção de post. Policy nunca é autorização suficiente: RBAC, source,
  snapshot, confirmação reforçada, executor e kill switch são revalidados no backend.
- `MetaAdsActionHistoryCard.vue`: mostra proposals/resultados autoritativos e os IDs
  confirmados de campaign/ad set/creative/ad; nunca infere sucesso pelo texto da IA.
- `MetaAdsAccountPicker.vue`: seleciona ad account cacheada e atual.
- `MetaAdsOverviewCard.vue`: KPIs agregados do cache autoritativo.
- `MetaAdsReportChart.vue`: série de insights; `vue3-apexcharts` é carregado em
  `<ClientOnly>` por import dinâmico.
- `MetaAdsCampaignTable.vue`: campanhas atuais, ainda sem editor manual.

Os antigos `MetaAdsAssistantAuth`, `MetaAdsAssistantCard`,
`MetaAdsAssistantMessage` e `MetaAdsAssistantSettings` foram removidos. Não os
reintroduzir nem voltar a consumir `/v1/meta-ads/assistant/*`; o contrato canônico é
`/v1/assistant/chat/*`.

## Store e concorrência

`useMetaAdsStore` mantém conexão, ad accounts, seleção, overview, campanhas, insights,
range, mapeamentos e permissões. Ações principais: `init`, `resetState`,
`cancelDataLoads`, `selectAdAccount`, `setRange`, `sync`, `startConnectionOAuth`,
`saveConnection`, `deleteConnection`, `setAdAccountClient` e
`setInstagramIdentityClient`.

Regras obrigatórias:

- toda request captura `accountId` e generation; resposta tardia nunca comita em outra
  conta;
- seleção de ad account e range usam gerações/AbortControllers próprios;
- troca de account, reset e unmount abortam requests e limpam dados sensíveis;
- permissões não resolvidas são fail-closed para `manage` e `connect`;
- o header de account deve ser o snapshot capturado no início da operação;
- `is_current` e connection ativa são garantias do backend; não montar fallback local
  para caches antigos.

Tipos vivem em `web/types/meta-ads.ts`; contratos de ações/policy ficam em
`web/app/domain/meta-ads`. Não duplicar shapes nos componentes.

## Contratos relevantes

- `POST /v1/meta-ads/oauth/start` inicia Facebook Login; callback público resolve a
  account exclusivamente pelo state persistido.
- `POST/DELETE /v1/meta-ads/connection` conecta/desconecta com `meta_ads.connect`.
- `GET /v1/meta-ads/ad-accounts`, `/campaigns`, `/insights`, `/overview` são leituras do
  cache atual; `POST /sync` exige manage.
- `PATCH /v1/meta-ads/ad-accounts/{id}/client` e
  `PATCH /v1/meta-ads/instagram-identities/{igUserId}/client` são agency-only.
- `GET/PUT /v1/meta-ads/ad-accounts/{id}/action-policy` lê/configura gates e caps.
- `/v1/meta-ads/action-proposals/*` é o lifecycle durável usado pelos cards do chat;
  não executar Graph pela UI.

## UI e estilo

- Usar tokens do design system, classes BEM-like e `<script setup lang="ts">`.
- Sem segredo, ID de token, resposta Graph crua ou mensagem que prometa write quando o
  executor/kill switch estiver indisponível.
- Cards devem distinguir claramente: disponível, proposta futura, pendente, executando,
  sucesso, falha, desconhecido, cancelado e expirado.
- Não sugerir que uma campanha foi criada/alterada antes de o backend devolver estado
  terminal autoritativo.

## Fora desta pasta

Store, tipos, composables compartilhados, host do chat, rotas, migrations, Graph client,
executor e wiring de RBAC pertencem às camadas correspondentes. Mudanças nesses contratos
exigem atualizar os AGENTs e o roadmap canônico no mesmo ciclo.
