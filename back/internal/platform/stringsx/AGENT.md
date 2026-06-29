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

// Regra canonica de slug: NFD + sem acentos (Mn) + hifen como separador + colapsa
// hifens + trim das pontas. Identica ao slugify() do front (domain/utils/slugify.ts).
// Exemplos: "Acao" -> "acao", "Perola@RioMar!" -> "perola-riomar".
stringsx.Slugify(raw)
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
- `Slugify` (decisao 2026-06-29): regra canonica unificada. Usa NFD (nao NFKD)
  para decompor acentos; descarta marcas Unicode Mn; hifen como separador. Modules
  `bio` e `cardapio` substituiram suas copias locais. `site.perolaSlug` e outra
  logica (usa `_`, especifica do crow-notion) e permanece inalterada. O front
  espelha a regra em `web/app/domain/utils/slugify.ts`. Slugs JA GRAVADOS no banco
  nao sao re-gerados; so novos slugs seguem a regra canonica.
- `crm/erp` mantem um alias local fino `firstNonEmpty` que delega para
  `stringsx.FirstNonEmpty`, so para nao reescrever os ~20 call-sites do pacote.

## Quando atualizar este AGENT.md

- Ao adicionar/remover uma funcao consolidada.
