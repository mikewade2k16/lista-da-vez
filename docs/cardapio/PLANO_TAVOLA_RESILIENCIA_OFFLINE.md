# PLANO — Resiliência offline do TAVOLA (site independente do painel)

> **Espelho no painel.** O plano canônico e as specs por agente vivem no repo do
> site: `TAVOLA/docs/resiliencia-offline.md` + `TAVOLA/docs/resiliencia-offline-specs.md`.
> Roadmap: fase `cardapio-online`, tasks `card-f14-*` (`data/phases-part5.ts`).
>
> **Status (2026-07-03): F1–F4 + publisher da F5 IMPLEMENTADOS** no repo TAVOLA
> (workflow de 5 agentes Opus; F4 consolidada pelo orquestrador; `npm run
> generate` PASS). Pendente: validação no browser (dono), primeiro deploy
> (`deploy:full` — SW novo) e, na F5, `SNAPSHOT_HOSTS` + cron na VPS. Deps novas
> no TAVOLA: `@vite-pwa/nuxt` + `typescript` (esta explicitada porque a
> re-resolução do npm removeu a transitiva e o build quebrava).

## Contexto

Incidente AC-04 (2026-07-03): o painel caiu e os sites de cardápio (TAVOLA no
HostGator) caíram junto, porque todo dado vem de `/v1/public/*` em runtime, sem
nenhum cache. Decisão do dono: **o site tem que continuar rodando com o que já
tem quando o painel cair** — só não recebe atualização até ele voltar.

## Desenho (resumo — detalhe no canônico)

Ordem de leitura: **API viva → snapshot local (localStorage, SWR com staleness
visível) → snapshot publicado no hosting (F5, opcional) → fallbacks atuais**.
Pedido com API fora: mensagem de WhatsApp montada localmente com aviso "valores a
confirmar" (o WhatsApp já é o canal real do pedido; sem fila de re-POST — risco de
duplicidade). Service Worker só para app shell + imagens `/uploads/*`
(stale-while-revalidate); **o SW não intercepta `/v1/*`** — cache de dado tem uma
única camada.

## Fases (specs prontas no repo TAVOLA)

| Fase | Escopo | Repo | Status |
|---|---|---|---|
| F1 | Snapshot local + SWR + banner de staleness (useTenant/useMenu/useSiteLayout/prato) | TAVOLA | pending |
| F2 | Pedido offline via WhatsApp (classificação de erro; 4xx nunca vira fallback) | TAVOLA | pending |
| F3 | Service Worker (`@vite-pwa/nuxt`): shell + imagens; `.htaccess` no-cache p/ `sw.js` | TAVOLA | pending |
| F4 | Endurecimento + matriz de QA + sync 3 docs + roadmap + panorama | TAVOLA + painel (docs) | pending |
| F5 (opcional) | Publisher de `_snapshot/<host>.json` no HostGator via cron na VPS | TAVOLA (`deploy/`) + infra VPS | pending |

## Impacto no painel / back Omni

- **Back: NADA obrigatório em nenhuma fase.** Os endpoints públicos já servem tudo
  (menu agregado, ETag no layout, `max-age=60` nos GETs).
- Melhoria futura opcional (não bloqueia): ETag/304 também no menu público
  (`GET /v1/public/restaurants/{slug}`) para baratear a revalidação do snapshot.
- F5 usa a VPS só como agendador (cron chamando o publisher com credencial FTP do
  HostGator). Se a VPS cair inteira, o último snapshot publicado continua no ar.

## Notas de Deploy

1. F1/F2/F4: só front TAVOLA (`npm run deploy`). Sem migration, sem rebuild da api,
   sem env nova.
2. F3: dep nova `@vite-pwa/nuxt` no TAVOLA (instalar só com aprovação) + `.htaccess`
   atualizado (vai no bundle). Primeiro deploy com SW: `npm run deploy:full`.
3. F5: `SNAPSHOT_HOSTS`/`SNAPSHOT_API_BASE` em `TAVOLA/deploy/deploy.env` + cron na
   VPS (comando exato definido na implementação, registrado aqui).

## Limitações aceitas (documentadas no canônico §6)

- Visitante novo durante a queda só é coberto pela F5.
- Imagens no first-visit em queda: placeholders (uploads moram no host do painel;
  storage independente é outra frente).
- Pedido do período offline não aparece na página de Pedidos do painel — o registro
  é a conversa de WhatsApp; a telemetria (`checkout_failed{api_offline}` +
  `whatsapp_order_clicked{offline:true}`) chega quando a API volta.
