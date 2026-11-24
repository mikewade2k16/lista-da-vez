# Observabilidade do Omni

Mede o sistema hoje e a cada melhoria, sem IA. Tem quatro partes:

| parte          | onde                                                                              | o que faz                                                                                      |
| -------------- | --------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Inventário     | `scripts/build-inventory.mjs` → `inventory.json`                                  | lista endpoints, páginas, tabelas, jobs e integrações lendo o código e o banco                 |
| Instrumentação | `back/internal/platform/observability`, `web/app/plugins/observability.client.ts` | grava toda requisição, consulta, chamada externa e evento do navegador no schema `obs`         |
| Runner         | `runner/` (`npm run obs:run`)                                                     | testa API, páginas, banco, carga e saúde; compara com a execução anterior e dá nota por módulo |
| Painel         | `/manage/observabilidade` (só `platform_admin`)                                   | mostra os dados ao vivo e as execuções                                                         |

Contratos entre as partes: [SPEC.md](SPEC.md). Fórmula da nota: [docs/SCORE.md](docs/SCORE.md).
Auditoria técnica: [reports/AUDITORIA.md](reports/AUDITORIA.md).

## Pré-requisitos

- Stack local de pé (`npm run dev:mac:up`). A API precisa ter aplicado a migration
  `0329_observability.sql`, o que acontece sozinho no boot do container.
- Node 24 e Google Chrome instalados. As páginas usam o Chrome do sistema via
  `playwright-core`, sem baixar navegador.
- `npm install --prefix observability` (uma vez).
- Credenciais de teste em `observability/.env.local` (ignorado pelo Git):

  ```bash
  OBS_USER_ADMIN_EMAIL=...
  OBS_USER_ADMIN_PASSWORD=...
  # opcionais, para o teste de autorização entre perfis:
  OBS_USER_CONSULTANT_EMAIL=...
  OBS_USER_CONSULTANT_PASSWORD=...
  ```

  Sem credencial, as rotas autenticadas ficam como `skip` com o motivo. Os testes
  negativos (sem token → 401) e as rotas públicas rodam mesmo assim. O runner nunca
  altera senha de ninguém.

## Como rodar

```bash
npm run obs:inventory                              # regenera inventory.json
npm run obs:inventory -- --sync                    # idem + grava em obs.inventory_items (módulo por rota no painel)
npm run obs:inventory -- --check                   # sai com 1 se o inventory.json estiver desatualizado

npm run obs:run                                    # tudo: api, pages, db, health
npm run obs:run -- --module=finance                # um módulo
npm run obs:run -- --only=api,db                   # só algumas fases (api|pages|db|load|health)
npm run obs:run -- --endpoint=/v1/finance          # endpoints que casam o texto
npm run obs:run -- --page=/manage/users            # uma página
npm run obs:run -- --load                          # inclui carga leve (autocannon)
npm run obs:run -- --repeat=10 --profile=ADMIN     # repetições por endpoint e perfil principal
npm run obs:run -- --baseline=20261123-051200      # compara com uma execução específica
npm run obs:run -- --no-fail-on-regression         # não falha o exit code por regressão
npm run obs:run -- --no-db-store                   # não grava em obs.runs

npm run obs:worker                                 # atende o botão "Rodar runner" do painel
npm --prefix observability run obs:compare -- <runA> <runB>
```

Cada execução fica em `observability/runs/<runId>/`:

- `results.json`, com tudo cru;
- `report.html`, estático, para abrir no navegador;
- `screenshots/`.

A execução também é gravada em `obs.runs` e `obs.run_results`.

Códigos de saída:

- `0`: ok;
- `1`: regressão acima do limite ou falha de autorização (serve para o CI);
- `2`: recusa de segurança ou erro de configuração.

## Segurança do runner

- Endpoints de escrita (POST/PUT/PATCH/DELETE) só rodam se todas as condições abaixo
  valerem:
  - `--allow-writes`;
  - `OBS_ALLOW_WRITES=1`;
  - `OBS_TEST_DATABASE=1`;
  - ambiente diferente de produção;
  - API e banco em localhost ou num host de staging declarado em `OBS_STAGING_HOSTS`.

  Se o ambiente parecer produção, o runner recusa com exit 2. Sem essas condições, a
  escrita aparece como "não testado — requer banco de teste".

- **O banco local do Mac é cópia de produção.** Não ligue escrita nele.
- GETs que disparam efeito (export, sync, OAuth, stream, websocket) ficam em
  `config.mjs → neverCall`, com o motivo. GETs que dependem de integração externa
  também são pulados por padrão (`OBS_SKIP_EXTERNAL=0` libera).

## Instrumentação: ligar, desligar e ajustar

| variável                                    | padrão         | efeito                                                                                                      |
| ------------------------------------------- | -------------- | ----------------------------------------------------------------------------------------------------------- |
| `OBSERVABILITY_ENABLED`                     | `true`         | `false` desliga tudo na API: sem middleware, tracer, coletor nem gravação                                   |
| `NUXT_PUBLIC_OBSERVABILITY_ENABLED`         | `true`         | `false` desliga o coletor do navegador                                                                      |
| `OBS_SAMPLE_RATE`                           | `1`            | fração das linhas brutas gravadas. 5xx, lentas e as do runner sempre são gravadas; os agregados contam 100% |
| `OBS_SLOW_QUERY_MS` / `OBS_SLOW_REQUEST_MS` | `200` / `1000` | limites de consulta lenta e de requisição lenta                                                             |
| `OBS_NPLUSONE_THRESHOLD`                    | `10`           | quantas vezes a mesma consulta numa requisição conta como suspeita de N+1                                   |
| `OBS_RETENTION_DAYS`                        | `30`           | apaga dado bruto mais velho; agregados e execuções ficam                                                    |
| `LOG_FORMAT`                                | `json`         | `text` volta ao log chave=valor                                                                             |

Nada sensível é gravado:

- não entram corpo de requisição nem de resposta, header de autenticação, valor de
  parâmetro SQL (a consulta é normalizada) nem query string;
- textos livres (erros, mensagens do front) passam pela sanitização central
  (`sanitize.go` e `web/app/utils/observability/sanitize.ts`), que mascara tokens,
  e-mail, CPF/CNPJ, telefone e cartão.

Correlation ID: o front envia `X-Request-ID: <pageViewId>-<seq>` em cada chamada e a API
reaproveita o valor. No painel, clique num correlation ID para ver a requisição, as
consultas lentas ou N+1, as chamadas externas e os eventos da página que a originou.

## Como ler o painel (`/manage/observabilidade`)

- **Visão geral:**
  - nota por módulo (0–100; verde a partir de 80, amarelo a partir de 60);
  - tendência das notas entre execuções;
  - cobertura;
  - números ao vivo: requisições, taxa de 5xx, p95 e eventos descartados (descarte
    acima de zero = buffer cheio).
- **Endpoints:** p50/p95/p99 calculados do histograma completo, taxa de erro, volume e
  queries por requisição. Ordene pelos piores, filtre por módulo.
- **Banco:**
  - consultas lentas;
  - suspeitas de N+1: mesma consulta repetida N vezes numa requisição, o que inclui
    inserts em loop;
  - top por tempo total;
  - alertas estruturais do último run: FK sem índice, índice duplicado ou sem uso,
    tabela sem PK.
- **Páginas:**
  - Web Vitals p75 dos usuários reais;
  - resultado do último run, medido em headless. No `nuxt dev`, LCP e tempo de carga
    ficam bem acima do que ficariam no build de produção.
- **Erros:** agrupados por assinatura, com os últimos correlation IDs.
- **Execuções:**
  - histórico de execuções;
  - comparação entre duas, com regressões em destaque;
  - botão para disparar o runner por módulo. Ele só funciona fora de produção e com
    `npm run obs:worker` rodando no host.

## Comparar antes × depois de uma melhoria

1. Antes de mexer: `npm run obs:run -- --module=<modulo>` e anote o `runId`.
2. Faça a melhoria e reinicie o serviço afetado.
3. `npm run obs:run -- --module=<modulo>`. A comparação com a execução anterior sai
   no console e no `report.html`, por exemplo "GET /v1/x p95 subiu de 120ms para 340ms".
4. Para comparar duas execuções quaisquer: `npm --prefix observability run obs:compare -- <antes> <depois>`,
   ou a aba Execuções do painel.

Compare execuções do mesmo escopo (mesmos `--module`/`--only`) e com a mesma carga de
máquina. Repetições maiores (`--repeat=10`) reduzem o ruído.

## Adicionar endpoints e páginas novos

O inventário sai do código, então não há cadastro manual:

- endpoint novo: registrar a rota com `mux.Handle("METODO /caminho", ...)`, como os demais;
- página nova: criar o `.vue` em `web/app/pages` ou `web/layers/*/pages`.

Depois rode `npm run obs:inventory -- --sync`, e o próximo `obs:run` já cobre o item.

Ajustes que ainda são manuais:

- módulo de um segmento de rota do front ou de um schema novo:
  `config/module-map.json`;
- GET que não pode ser chamado: `config.mjs → neverCall`, sempre com o motivo;
- limite de latência específico: `config.mjs → limits`.

Use `npm run obs:inventory -- --check` no CI para barrar inventário desatualizado.

## Segurança

**A cobertura de segurança ainda é baixa.** O catálogo (`config/security-catalog.json`)
lista 81 ataques e controles (OWASP Top 10 2021, OWASP API Top 10 2023 e outros), mas só
22 têm teste automático. Os demais aparecem no painel como `not_tested`, e isso não
significa que estejam seguros. Uma nota boa aqui não prova ausência de falhas.

**Testado automaticamente** (fase `security`, só leitura, sem ataque destrutivo):

- headers de segurança da API e do web, clickjacking, CORS com `Origin` malicioso;
- flags do cookie de sessão e onde o front guarda o token;
- JWT adulterado e com `alg none`; rotas protegidas sem token (reaproveita a fase `api`);
- isolamento entre tenants em leitura (exige duas contas, veja abaixo);
- enumeração de usuário no login e 403 x 404 para conta fora do escopo;
- SQL injection básica (`' OR '1'='1` e `pg_sleep`) em cerca de 20 GETs e XSS refletido básico;
- listagem de `/uploads` e path traversal; corpo de 5 MB; open redirect no servidor;
- segredos nos arquivos versionados (`git ls-files`, só `arquivo:linha`, nunca o valor);
- dependências (reaproveita `npm audit` e `govulncheck` da fase `health`); TLS com URL https.

**Não testado automaticamente:** XSS armazenado, CSRF, IDOR em escrita, mass assignment,
SSRF dinâmico, upload malicioso, webhooks sem assinatura, prompt injection nos assistentes
de IA, logs com dados sensíveis, MFA, revogação de sessão, expiração de JWT, segredos no
histórico do Git, infra da VPS e LGPD. O redirecionamento pós-login feito no front também
não é verificado (`open-redirect` fica `partial`).

**Como rodar:**

```
npm run obs:security                          # só imprime o que faria
npm run obs:security -- --sync                # grava catálogo e achados da AUDITORIA no banco
npm run obs:security -- --dry-run-db          # idem, numa transação com rollback
npm run obs:run -- --only=security            # executa as checagens (grava em obs.security_checks)
npm run obs:run -- --only=api,health,security # reaproveita authz, npm audit e govulncheck
```

O `--sync` não sobrescreve `status`, `detail`, `evidence` nem `checked_at` de checagem já
testada, nem o `status` de um achado já editado. Sem `--only`, a fase `security` roda por
último no padrão.

**Cuidados:**

- Em produção rodam só as passivas (headers, CORS, cookies, TLS, repositório e dependências).
  As demais viram `not_tested` com o motivo.
- Força bruta (`login-bruteforce`), corpo grande e enumeração de usuário rodam por último,
  só fora de produção, para um eventual 429 no IP do runner não afetar as outras fases.
  A força bruta usa e-mail gerado `obs-bruteforce-<aleatório>@example.invalid`, nunca e-mail real
  (`OBS_BRUTEFORCE_ATTEMPTS`, padrão 30). A enumeração faz uma única tentativa com senha errada
  no usuário ADMIN. Toda requisição respeita o `rateLimitPerMin` do runner.
- Isolamento entre tenants: defina `OBS_USER_TENANT_A_EMAIL/PASSWORD` e
  `OBS_USER_TENANT_B_EMAIL/PASSWORD` no `.env.local` (contas de tenants diferentes). Sem elas
  a checagem fica `not_tested` ("faltam credenciais de duas contas diferentes").
- O CI roda `go vet` e `go test -race` em `back/` (`.github/workflows/ci-race.yml`).

## Deploys

O histórico de deploys vive em `obs.deploys` e aparece na aba **Deploys** do painel (mediana,
p90, taxa de sucesso, veredito ok/lento/falhou e etapas de cada deploy). A medição é o tempo
de ponta a ponta: do início do comando na máquina até os smoke tests passarem na VPS.

- Ao fim de cada `npm run deploy`, `scripts/deploy/run-deploy.mjs` grava a linha no banco do
  ambiente alvo (mesma conexão SSH do deploy). Falha só gera aviso; `OBS_DEPLOY_RECORD=0` desliga.
- Importar os já feitos (`.tmp/deploy/*/summary.json`): `npm run obs:deploys` lista;
  `npm run obs:deploys -- --sync` grava no banco local (`--target=prod|staging` usa o SSH).
  É idempotente por id. `--dry-run-db` testa tudo numa transação com rollback.

## Uptime externo

`.github/workflows/uptime.yml` roda a cada 5 minutos no GitHub Actions (fora da VPS, por
isso pega o caso de a maquina inteira cair com o monitor interno). Chama
`https://omni.crowvisuals.com.br/healthz` e a home, com retry curto. Se falhar, abre uma
issue com a label `uptime` (ou comenta na que ja esta aberta, sem duplicar); quando volta,
fecha a issue com o comentário "voltou". Usa só o `GITHUB_TOKEN`, sem segredos; também
pode ser disparado à mão (`workflow_dispatch`). Ative as notificações de issues do
repositório para receber o aviso.

## Reverter

A migration é aditiva: só cria o schema `obs`. Para remover:
`psql "$DATABASE_URL" -f observability/migrations/0329_observability.down.sql`. Isso
apaga só a telemetria e a linha de controle da migration.
