# AGENT — platform/stringsx

## Escopo

Pacote `back/internal/platform/stringsx/`. Helpers de string compartilhados
entre modulos, para acabar com copias identicas espalhadas pelo back.

## Por que existe

Varios modulos (queue, crm, cardapio, site, users) carregavam copias byte-a-byte
das MESMAS funcoes utilitarias. Consolidamos as que tinham semantica identica num
ponto unico (fonte unica de verdade), sem alterar comportamento observavel.

## API atual

```go
// Primeiro valor nao-vazio (apos TrimSpace), ja trimado. Tudo vazio => "".
stringsx.FirstNonEmpty(a, b, c)

// Trim + descarta vazios + dedup preservando a ordem. Sempre slice nao-nil.
stringsx.NormalizeIDs(storeIDs)

// jsonb (text[]/json serializado) -> []string. Vazio/invalido => slice nao-nil vazio.
stringsx.DecodeJSONStringSlice(rawBytes)
```

## Convencoes

- **Refactor puro**: ao mover um helper para ca, o comportamento OBSERVAVEL de
  cada call-site deve ficar identico. `FirstNonEmpty` devolve o valor trimado —
  isso cobre tanto as variantes "trimmed" antigas quanto as "raw" que sempre
  eram consumidas dentro de um `strings.TrimSpace(...)` externo.
- **Sem dependencia de dominio**: este pacote so depende da stdlib. Nada de
  tipos de modulo aqui.
- **Testado**: `stringsx_test.go` cobre cada funcao consolidada. Toda nova func
  entra com teste.

## Notas

- `DecodeJSONStringSlice` sempre devolve `[]` nao-nil (nunca `nil`). Isso mantem
  identico o comportamento antigo do cardapio (Gallery/Diet/Allergens/Tags ja eram
  `[]`) e PADRONIZA operations/reports: os campos `VisitReasons`/`CustomerSources`/
  `LossReasons`/`SiblingServiceIDs`, que antes podiam sair como `null` quando o
  jsonb armazenado fosse literalmente `null`, agora saem como `[]`. Mudanca
  deliberada (decisao 2026-06-29): inocua para o front (trata `null` e `[]` como
  lista vazia) e mais consistente. Unica divergencia de saida assumida na
  consolidacao.
- `slugify`/`normalizeSlug` NAO foram consolidados: as 6 copias geram slugs
  DIFERENTES (NFKD vs. drop de acento vs. so-lowercase) e unificar mudaria o
  slug gerado — precisa de decisao de produto sobre qual regra e canonica.
- `crm/erp` mantem um alias local fino `firstNonEmpty` que delega para
  `stringsx.FirstNonEmpty`, so para nao reescrever os ~20 call-sites do pacote.

## Quando atualizar este AGENT.md

- Ao adicionar/remover uma funcao consolidada.
- Quando o slug ganhar uma regra canonica e puder ser movido para ca.
