# Nota por módulo (0–100)

`nota = 100 − (erros + latência + vitals + banco + cobertura)`, arredondada. Implementação:
`runner/score.mjs` (a fórmula também está no comentário do topo do arquivo). Pesos em
`config.mjs` (`scoreWeights`).

| componente | máx | cálculo |
|---|---|---|
| erros | 30 | `min(30, 10 × falhas authz + 5 × falhas 5xx/rede/navegação + 3 × demais falhas)` (falhas `fail` de api, page, authz, db, load do módulo) |
| latência | 25 | `25 × média` sobre os GETs medidos do módulo de `clamp((p95/limite − 1)/2, 0, 1)`; no limite = 0, 3× o limite ou mais = penalidade cheia |
| Web Vitals | 20 | `20 × média` sobre as páginas medidas da fração de métricas fora do limite (LCP 2500, INP 200, CLS 0,1, TTFB 800: acima = 0,5; acima de 2× = 1) |
| banco | 15 | `min(15, 0,5 × FK sem índice + 0,5 × índice duplicado + 1 × tabela sem PK + 1 × query lenta + 2 × N+1 + 1 × Seq Scan grande)` só itens atribuídos ao módulo (tabela → módulo do inventário; rota → módulo do endpoint) |
| cobertura | 10 | `10 × (1 − média dos % testados)` de endpoints, páginas e tabelas, apenas das fases executadas no run |

- Módulo sem nenhum resultado medido (tudo `skip`) → `score: null` ("sem dados"), nunca 100.
- Limites de p95: padrão 500 ms; sobrescrevíveis por módulo/endpoint em `config.limits`.
- Runs com filtros (`--module`, `--only`) geram notas só do que rodou; compare apenas runs de mesmo escopo.
- Itens `db:*` sem módulo (checagens globais) não entram na nota de módulo; aparecem no relatório.
