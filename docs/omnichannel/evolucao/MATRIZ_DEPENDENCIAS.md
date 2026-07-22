# Matriz de dependências, ownership e integração

## 1. Grafo executivo

```text
E0
└── E1 ──┬── E2 ──┬── E3
         │        ├── E5 ──┐
         │        └── E6 ──┤
         └── E4 ───────────┤
                            └── E7 ── E8
E9 acompanha cada entrega ───────────────┤
                                         └── E10
```

E9 não é “uma faxina no fim”: cada pacote aplica seus requisitos locais; a fase E9 fecha os
controles transversais e prova escala/recuperação.

## 2. Matriz de fases

| Fase | Depende de | Pode paralelizar | Owner de integração | Gate de saída |
|---|---|---|---|---|
| E0 | — | nada que altere n8n | arquitetura/devops | owners e guardas provados |
| E1 | E0 | E4 após contrato de contato | backend omnichannel | smoke WhatsApp completo em UI |
| E2 | E1 | E4 | IA/orquestração | shadow + decisão versionada idempotente |
| E3 | E2 | partes de E5 | mídia/IA | áudio/imagem/documento com limites e fallback |
| E4 | E1 | E2 | CRM/backend | contato 360° e atribuição pesquisáveis |
| E5 | E2, E4 | E3 | domínio/inbox | handoff sem corrida IA×humano |
| E6 | E2, E4 | E3/E5 parcial | tools/IA | tool allowlisted, auditada e tenant-safe |
| E7 | E1–E6 | hardening E9 | canais/backend | um número oficial em canário, rollback provado |
| E8 | E4–E7 | E9 | canal Instagram | DM/comentário no mesmo domínio e inbox |
| E9 | transversal | todos com coordenação | plataforma/security | SLO, retenção, carga, backup e restore |
| E10 | E1–E9 | — | release/negócio | piloto e expansão com critérios objetivos |

## 3. Áreas de escrita por fase

`R` = leitura; `W` = escrita autorizável por pacote explícito; `—` = fora de escopo.

| Área | E0 | E1 | E2 | E3 | E4 | E5 | E6 | E7 | E8 | E9 | E10 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `back/internal/modules/omnichannel` | R | W | W | W | W | W | W | W | W | W | R |
| migrations `messaging.*` | R | W | W | W | W | W | W | W | W | W | R |
| `web/.../omnichannel` | R | W | W | W | W | W | W | W | W | W | R/W flag |
| `workflow-omnichannel-brain.json` | R | — | W | W | — | — | W | — | — | R | R |
| `workflow-instagram-first-contact.json` | R | — | — | — | — | — | — | — | W | R | R |
| scripts n8n compartilhados | W guardas | — | — | — | — | — | — | — | — | R | — |
| outros workflows/WAHA | — | — | — | — | — | — | — | — | — | — | — |
| plataforma `jobs/llm/secretbox/logmask` | R | W mínimo | W mínimo | W mínimo | — | W mínimo | W mínimo | W mínimo | W mínimo | W | R |
| compose/deploy/backup | R | — | — | — | — | — | — | W doc/env | W doc/env | W | W runbook |

`W mínimo` só vale quando o pacote lista arquivo e prova que a abstração é compartilhada. Sem
allowlist explícita, a área continua somente leitura.

## 4. Reserva de migrations

O maior prefixo observado na criação destas specs é `0212`, mas esse fato expira quando outro
módulo criar migration. O orquestrador mantém uma única fila de reserva:

| Ordem lógica | Fase | Conteúdo esperado | Número |
|---:|---|---|---|
| 1 | E1 | status/reply/media ingest que ainda não couber no schema | reservar no dispatch |
| 2 | E2 | buffer/dispatch/decisão IA | reservar após E1 |
| 3 | E3 | análises de mídia | reservar após E2 |
| 4 | E4 | complementos CRM/merge/landing | reservar após auditoria de `0211/0212` |
| 5 | E5 | handoff/SLA | reservar após E4 |
| 6 | E6 | tool definitions/runs/knowledge | reservar após E5 |
| 7 | E7 | Meta accounts/templates/policy | reservar após E6 |
| 8 | E8 | Instagram accounts/comments/actions | reservar após E7 |
| 9 | E9 | controles transversais que não existirem | reservar por pacote |

Não criar migration vazia para “guardar número”. A reserva é registrada no pacote; se houver
colisão antes do merge, o orquestrador renumera antes de qualquer ambiente compartilhado aplicar.

## 5. Contratos compartilhados que têm um único owner

| Contrato | Owner primário | Consumidores | Regra de mudança |
|---|---|---|---|
| envelope `brain.request.v2`/`brain.result.v2` | E2 | E3/E5/E6/E8 | aditivo ou nova versão; nunca quebrar v2 em silêncio |
| identidade de contato | E4 | E5/E6/E8 | E8 adiciona provider, não cria outro CRM |
| política de envio | E7 | E1/E5/E8 | adapter informa capability; service decide |
| estado da conversa | domínio existente/E5 | todos | ampliar só com migration + matriz + testes completos |
| análise de mídia | E3 | E2/E5/E6 | resultado imutável por hash/versão |
| auditoria/custo | E9 | todos | produtores escrevem; E9 agrega/retém |
| outbox | backend existente | todos os canais | nenhum canal cria fila paralela |

## 6. Regras de merge e revisão

1. DB entra antes do backend que a consome.
2. Backend aceita ausência do front novo; deploy deve ser backward compatible.
3. Front só entra depois de contrato HTTP estabilizado e fixture/teste de resposta.
4. Workflow entra desativado/shadow antes da policy permitir resposta.
5. Feature flag é aplicada no Go; UI apenas reflete capability/estado.
6. Dois pacotes não editam o mesmo arquivo simultaneamente. O orquestrador divide por arquivo ou
   serializa.
7. `module.go`, wiring global, migrations e workflows são pontos de integração serializados.
8. Revisor compara hash/diff dos workflows não pertencentes ao módulo antes e depois.

## 7. Gate para despachar um pacote

Um pacote está `READY` somente se tiver:

- dependências `VERIFIED` ou mocks contratuais explicitamente autorizados;
- decisão de produto fechada;
- allowlist de leitura/escrita;
- migration reservada quando aplicável;
- request/response ou evento com versão;
- testes exatos e dados de fixture;
- critério observável em UI/API/banco;
- rollback local;
- revisor diferente do executor.
