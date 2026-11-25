# Progresso — módulo de observabilidade (branch feat/observabilidade)

Arquivo de retomada: se a sessão cair, continuar a partir daqui.
Contratos: `observability/SPEC.md`.

## Estado do ambiente (2026-11-23)
- Docker Desktop estava desligado; iniciado com `open -a Docker`. Stack omni de pé.
- Disco do Mac: ~3,7 GB livres. Screenshots em JPG q50; apagar perfis temporários do Chrome.
- Banco local é CÓPIA DE PRODUÇÃO (compose.mac-local.yaml): runner NÃO roda escrita nele.
- Credenciais de teste: o runner lê `OBS_USER_<PERFIL>_EMAIL/PASSWORD` (fallback
  `OMNI_QA_EMAIL/PASSWORD`). Ainda não fornecidas pelo usuário. Nunca resetar senha.
- A branch saiu de fix/finance com ~130 arquivos staged de outra frente: commitar SEMPRE
  com caminhos explícitos (`git commit -- <paths>`), nunca `-a`/`add -A`.

## Fases
- [x] F0 inventário (919 endpoints, 80 páginas, 396 tabelas) — commit 9912e3d
- [x] F1 auditoria (`observability/reports/AUDITORIA.md`): 1 crítico (XSS comentários tasks, confirmado), 6 altos, 17 médios, 8 baixos
- [x] F2 instrumentação Go + front + migration 0329 (API rebuild feito; obs gravando; testes e golangci ok; front sincronizado no omni-web-1)
- [x] F3 runner — commits 9170333, 615f94a
- [x] F4 página `/manage/observabilidade` (código pronto; falta validar com dados reais)
- [x] F5: execuções autenticadas 081550/082204 (AUDITORIA §8), painel validado com dados reais — commit 84928d9. Sem push.
- Disco: `docker builder prune -f` liberou 20 GB (cache de build), usar se apertar de novo.

## Fase 6 — Saúde do sistema (2026-11-23, autorizada: "go go, não precisa perguntar")
Contratos: SPEC.md seção "Saúde do sistema (fase 6)". Migration 0330 (+ down) escrita e validada (rollback).
Alertas: só e-mail agora (OBS_ALERT_EMAILS); WhatsApp explícito como "planejado".
Segurança: painel deve dizer honestamente que a cobertura é baixa (testado x não testado).
Agentes (sonnet) em paralelo, arquivos disjuntos:
- [x] A1 coletor (platform/observability/system*.go, presence, retenção)
- [x] A2 alertas + mailer + endpoints /health /security /system /alerts (modules/observability, platform/mailer)
- [x] B obs-agent (back/cmd/obs-agent, internal/obsagent, Dockerfile, compose)
- [x] C front (aba Saúde padrão, Sistema, Segurança, Alertas, heartbeat)
- [x] D runner security + catálogo + security-sync + ci-race.yml
Integração (orquestrador): build/lint/test, rebuild api (+ obs-agent), obs:security --sync, obs:run --only=security, validar página, commit explícito (sem cardápio: product-sort.ts, CardapioProductMobileToolbar.vue, cardapio/AGENT.md, product-sort.test.ts ficam fora), sem push.

- [x] E deploys + F ajustes (container_down por restart policy, pool sem emptyAcquire, 5xx só live). Commit 6d2619a, sem push. Pendente: deploy na VPS (migration 0330 + obs-agent), SMTP/OBS_ALERT_EMAILS no .env de prod, credenciais TENANT_A/B para teste entre contas.

## Fase 7 — correção dos críticos/altos (2026-11-23, autorizado: "vamos começar pelos críticos ... multiagentes")
Feito por mim (não commitado): timeout de 20s no useObsQuery; tipo ModulePathGuard unificado (fim do "Duplicated imports");
cpu_shares no compose.mac-local (web 4096; api/postgres/local-access 2048) + docker update aplicado; docs/SETUP_MAC.md;
pacote back/internal/platform/htmlsanitize (bluemonday v1.0.27 no go.mod).
Agentes (sonnet), builds com --cpus=2 --cpu-shares=256 e volume omni_gocache:
- [x] G1 XSS C-01 (sanitizar rich text gravação+leitura, varrer v-html)
- [x] G2 vulnerabilidades Go (go.mod/go.sum, imagens golang, govulncheck no CI)
- [x] G3 /uploads sem listagem + arquivos sensíveis; B-01 403→404
- [x] G4 login brute force + rate limit por identidade + IP confiável (A-01)
- [x] G5 graceful shutdown + recover workers (A-03)
- [x] G6 500→400 em catalog/analytics/store-staff/alerts
- [x] G7 CTE known_orders (operations/snapshot) + migration 0331 se índice
- [x] G8 npm audit web (sem major)
Depois: integração (build/test completo, rebuild api, runner security), commit explícito (cardápio separado), sem push.
Fora por ora: cookie de sessão HttpOnly (exige CSRF; refatoração de auth).

- Integração fase 7 OK (2026-11-23): go build/vet/test completos (golang:1.26.11), precommit:check verde, vitest 785. NÃO commitado; parado a pedido do usuário antes da rodada de segurança/rebuild da API.
