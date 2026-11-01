# Padrões de lint — Go, SQL, TypeScript/Vue

Leia ao escrever código nessas linguagens. Todo código gerado já deve respeitar estas regras — o linter as cobra no pré-commit, e auditar um arquivo geralmente revela erros pré-existentes do pacote inteiro. Escrever certo na primeira vez evita commits de fixup.

## O pre-commit roda 4 checks e PARA no primeiro que falha

`lint-staged` (via husky) despacha, nesta ordem: **ESLint+Prettier** no `web/`, **gofmt+golangci-lint** no `back/`, e o **linter de migrations** nos `.sql`. Como para no primeiro que falha, corrigir só o erro que apareceu faz o *próximo* `git commit` revelar o check seguinte — dá a sensação de "os erros não acabam". Ao consertar lint, **valide os quatro antes de recomitar**:

```bash
# web (ESLint, como o hook — max-warnings=999, só errors bloqueiam)
cd web && npx eslint --max-warnings=999 <arquivos web/ staged relativos a web/>
# Go (golangci nos PACOTES dos .go staged; --new-from-rev=HEAD, só issues novas)
bash scripts/dev/lint-go-staged.sh <arquivos .go staged>
# migrations (DDL sem schema qualificado)
bash scripts/dev/lint-migrations-staged.sh <arquivos migrations/*.sql staged>
```

Dois pontos que mordem: (1) o `lint-staged` linta o que está **staged** — depois de corrigir, `git add` de novo, senão o hook roda sobre a versão antiga; (2) toda vez que um lint novo bloquear um commit, **adicione o padrão a este arquivo** — a skill cresce com cada lint real encontrado, não só com os genéricos.

### Dependências do web incompatíveis entre Linux e Windows

Se o hook no Git Bash informar que `eslint`/`prettier` não é reconhecido, ou se o ESLint falhar com `Cannot find native binding` em `unrs-resolver`, o `web/node_modules` foi instalado para outra plataforma (por exemplo, no container/WSL e depois reutilizado pelo host Windows). Não copie nem compartilhe `node_modules` entre plataformas. Recrie a instalação no mesmo ambiente que executará o hook:

```bash
npm ci --prefix web
```

Os wrappers web chamam os entrypoints JavaScript diretamente para não depender dos links de `web/node_modules/.bin`, que também podem ser incompatíveis entre Linux e Windows.

## Go

| Situação | Errado | Certo |
|---|---|---|
| Permissão de diretório (`os.MkdirAll`) | `0o755` | `0o750` |
| Permissão de arquivo (`os.WriteFile`) | `0o644` | `0o600` |
| if/else-if com 3+ ramos booleanos | `if a {} else if b {} else if c {}` | `switch { case a: ... default: ... }` |
| Switch em variável simples | `if x == "a" {} else if x == "b" {}` | `switch x { case "a": ... }` |
| Conversão redundante de tipo | `http.HandlerFunc(h)` quando `h` já é `http.HandlerFunc` | `h` direto |
| Request em teste sem context | `httptest.NewRequest(...)` | `httptest.NewRequestWithContext(context.Background(), ...)` |
| Código morto / tipo não usado | declarar struct/func sem usar | apagar ou não criar |
| Falso-positivo de linter de segurança já verificado | — | comentário `//nolint:...` na linha específica |
| Upload multipart/form (gosec **G120** "unbounded form parsing") | `r.ParseMultipartForm(n)` sem teto do corpo | envolver `r.Body = http.MaxBytesReader(w, r.Body, max)` **antes** e marcar a linha do parse: `//nolint:gosec // G120: corpo limitado por MaxBytesReader acima` (o gosec não rastreia o MaxBytesReader e reclama mesmo com o teto) |
| Linha em branco no topo/fim de bloco (**whitespace**) | `if x != nil {`⏎`⏎    ...` | corpo do bloco começa e termina sem linha em branco extra colada à `{`/`}` |

Sempre `gofmt`. Linters típicos: análise de segurança (perms de arquivo/dir, bounds de slice), estilo (cadeia if-else), staticcheck, conversões redundantes, request sem context, símbolos não usados.

## SQL — schema sempre qualificado

| Situação | Errado | Certo |
|---|---|---|
| Tabela em `public` | `drop table if exists user_store_roles` | `drop table if exists public.user_store_roles` |
| Tabela em schema próprio | `create table consultants (...)` | `create table queue.consultants (...)` |
| Referência de FK | `references users(id)` | `references public.users(id)` |

O Postgres resolve nomes sem schema pelo `search_path` (que pode mudar); schema explícito garante execução igual em qualquer ambiente. Um linter de migrations costuma bloquear o commit ao achar DDL sem schema qualificado.

## TypeScript / Vue

| Situação | Errado | Certo |
|---|---|---|
| Regex com caractere de controle | `/.../g` puro | comentário `eslint-disable-next-line no-control-regex` na linha anterior |
| `console.log` em produção | `console.log(x)` | `console.warn(x)` ou `console.error(x)` |
| Arquivo grande (>500 linhas) | arquivo único gigante | dividir em composables/componentes menores |
| Variável declarada e não usada | `const x = ...` sem uso | apagar ou prefixar com `_` |
| `any` explícito | `param: any` | tipo específico ou `unknown` |
| Two-way binding numa prop (**vue/no-mutating-props**) | `v-model="prop.campo"` | quando o pai muta o objeto **por referência** (passa `draft.linha` e lê de volta após editar): `const m = toRef(props, 'prop')` + `v-model="m.campo"` — mesmo objeto, sem mutar a prop pelo nome; senão emitir `update:` |
| `<template>` só com comentário (**vue/valid-template-root**) | `<template><!-- markup --></template>` | um SFC exige 1 elemento raiz real; ocultar um bloco com `v-if="false"`/flag nomeada, **nunca comentar o `<template>` inteiro** (apodrece e quebra o lint) |
| U+FEFF/BOM fora de string (**no-irregular-whitespace**) | BOM invisível em comentário ou código | BOM só dentro de string literal (ex.: prefixo de CSV p/ Excel); em comentário escreva `U+FEFF`, não o caractere |

Linters típicos: caractere de controle em regex, sem `console` (permite warn/error), limite de ~500 linhas, imports/vars não usados, sem `any` explícito, mutação de prop, template root válido, whitespace irregular.
