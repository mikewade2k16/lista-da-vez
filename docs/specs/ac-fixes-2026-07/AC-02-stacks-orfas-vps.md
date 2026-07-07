# AC-02 — Runbook: higiene das stacks órfãs da VPS (crash-loop + omni-crow/omni-staging)

> **Tipo:** RUNBOOK MANUAL. Nenhum agente acessa a VPS — **o usuário executa** os comandos
> por SSH. A única mudança de arquivo neste repo é documentar o runbook numa seção nova
> de `docs/DEPLOY_VPS.md` (seção "Higiene da VPS"), para que ele fique versionado e
> re-executável.

## 1. Contexto

**Achado canônico AC-02 (P0, esforço S, impacto alto)** — fonte: `fatos.json → achados_canonicos.AC-02` e `infra.stacks_orfas`:

- VPS `85.31.62.33` (srv1507028, Ubuntu 24.04, ~6GB RAM, user `deploy`).
- Stack **`omnichannel-mvp`** em `/opt/omnichannel`: postgres morto (`Exited 255`) derrubou
  `plataforma-api` e `whatsapp-evolution-gateway` em **crash-loop infinito** (~78k/74k
  reinícios, `restart: unless-stopped`), contribuindo para `dockerd` a 158% de CPU.
- **PORÉM o Caddy dessa stack (`omnichannel-mvp-caddy-1`, `/opt/omnichannel/Caddyfile`)
  roteia o painel `omni.crowvisuals.com.br`** (e também `lista.whenthelightsdie.com`,
  `crowvisuals.com.br` → `crow-web`, `preview.whenthelightsdie.com` → `lista-staging-*`).
  Evidência: `docs/DEPLOY_VPS.md:145-176` ("Proxy reverso (Caddy compartilhado)").
  **NUNCA parar/remover o Caddy** — derruba o painel inteiro.
- Há ainda 2 stacks órfãs sem nenhum domínio apontando para elas: **`omni-crow-*`** (up há
  semanas, ocupando `127.0.0.1:18081/13004` — colisão documentada em
  `docs/deploy/STAGING_SETUP.md:203-209`) e **`omni-staging-*`** (exited, resíduo de
  tentativa antiga de staging).

**Contexto adicional verificado (auditorias anteriores na VPS, 2026-06-16/22):**

- A stack real do painel se chama **`listaatendimento-*`** (`COMPOSE_PROJECT_NAME=listaatendimento`
  em `/home/deploy/lista-atendimento`), não `omni-*` como partes de `docs/DEPLOY_VPS.md` sugerem.
- O container **`crow-web`** serve `crowvisuals.com.br` (site/bio) e **NÃO pertence** à stack
  órfã `omni-crow` apesar do nome parecido — não pode ser parado.
- Parte da limpeza **já foi executada em 2026-06-22** (crash-loops parados com
  `docker update --restart=no` + `docker stop`; do omnichannel-mvp só o caddy segue de pé).
  A causa raiz do pico de CPU foi um agravante extra: **comandos `docker logs` órfãos**
  (sessão SSH caída, reparented a PPID 1) presos no `/run/docker.sock`. O runbook precisa
  ser **idempotente**: diagnóstico primeiro; só age no que ainda estiver fora do lugar.

Por que importa: crash-loops e streams órfãos consomem CPU do `dockerd` numa VPS de ~6GB
dividida com a produção real da Pérola; e sem runbook versionado a limpeza depende de
memória de sessão — qualquer recaída (reboot da VPS ressuscita `restart: unless-stopped`
de container não neutralizado, novo `docker logs` órfão) volta a degradar o painel.

## 2. Objetivo e não-objetivos

**Objetivo:**

1. Documentar em `docs/DEPLOY_VPS.md` uma seção nova **"Higiene da VPS (stacks órfãs)"**
   com o runbook completo (diagnóstico → neutralizar → validar → rollback), pronto para o
   usuário colar na VPS.
2. Resultado esperado quando o usuário executar: CPU do `dockerd` normalizada (<5%),
   load <1, zero containers em restart-loop, e painel `omni.crowvisuals.com.br` no ar.

**Não-objetivos (explicitamente FORA de escopo):**

- **NÃO** parar/remover/reiniciar `omnichannel-mvp-caddy-1` em nenhuma hipótese.
- **NÃO** tocar em `crow-web`, `listaatendimento-*` (api/web/postgres/n8n/waha/redis) nem
  no staging legítimo de `/home/deploy/lista-atendimento-staging`.
- **NÃO** remover volumes (`docker volume rm`, `docker compose down -v`) — dados das
  stacks órfãs ficam preservados para eventual resgate.
- **NÃO** rodar `docker compose down` em `/opt/omnichannel` (removeria o caddy junto).
- **NÃO** migrar o roteamento para um Caddy próprio da stack `listaatendimento` — isso é
  o item futuro **IN-01** (infra), referenciado mas não executado aqui.
- **NÃO** acessar a VPS por nenhum agente; nenhum script automatizado — só doc.
- **NÃO** alterar código Go/TS, compose, CI ou migrations.

## 3. Mudanças

### 3.1 Único arquivo a editar: `docs/DEPLOY_VPS.md`

Inserir a seção abaixo **imediatamente ANTES** da linha
`## Deploy via GitHub Actions (alternativa aos scripts locais)` (hoje linha ~209, logo
após o bloco "Variaveis principais do `.env.production`" e o `---` que o encerra).

Regras da edição:

- Manter o **estilo sem acentos** do doc (ele usa "nao", "e'", "orfa" de propósito).
- Inserir o bloco EXATAMENTE como abaixo (entre os marcadores `<<<INICIO` / `FIM>>>`,
  que não entram no arquivo). Orçamento: ~105 linhas — `DEPLOY_VPS.md` tem 336 e fica
  em ~441 (dentro do teto de 450).

<<<INICIO DO BLOCO A INSERIR

```markdown
## Higiene da VPS (stacks orfas) — runbook manual

> Executado em 2026-06-22 (crash-loops do omnichannel-mvp parados); re-executavel a
> qualquer momento (idempotente): o diagnostico decide o que ainda precisa de acao.
> TUDO aqui e' manual, via SSH: `ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33`

**Mapa de containers — quem NUNCA pode ser tocado vs. alvos:**

| Protegidos (NUNCA parar/remover) | Por que |
|---|---|
| `omnichannel-mvp-caddy-1` | proxy de TUDO: painel omni.crowvisuals.com.br, lista./preview.whenthelightsdie.com, crowvisuals.com.br |
| `listaatendimento-*` (api, web, postgres, n8n, waha, redis) | a producao real do painel |
| `crow-web` | serve crowvisuals.com.br; NAO e' da stack omni-crow apesar do nome |
| staging de `/home/deploy/lista-atendimento-staging` (se up) | staging legitimo sob demanda |

Alvos: containers `omnichannel-mvp-*` EXCETO o caddy (postgres morto + crash-loops +
satelites) e as stacks orfas `omni-crow-*` (ocupava 127.0.0.1:18081/13004) e
`omni-staging-*` (residuo de staging antigo). Consequencia conhecida e aceita:
`app./api./db.whenthelightsdie.com` ficam 502 (ja estavam quebrados desde ~maio/2026).

### H1. Snapshot (seguro de rollback) + diagnostico

```bash
mkdir -p ~/higiene-vps && cd ~/higiene-vps
docker ps -a --format '{{.Names}}\t{{.Status}}\t{{.Image}}' | tee ps-$(date +%Y%m%d-%H%M).txt
docker ps -aq | xargs docker inspect -f '{{.Name}} restart={{.HostConfig.RestartPolicy.Name}} status={{.State.Status}} restarts={{.RestartCount}}' | tee policies-$(date +%Y%m%d-%H%M).txt

top -bn1 | head -15                          # load e %Cpu(s)
ps -C dockerd -o pid,pcpu,pmem,etimes,args   # CPU do dockerd (alvo: <5%)
docker stats --no-stream

# clientes presos no docker.sock (causa raiz do incidente de 2026-06-22):
ss -xp 2>/dev/null | grep docker | head
ps -eo pid,ppid,etimes,args | grep -E 'docker (logs|exec)' | grep -v grep
# processo `docker logs` com etimes gigante e PPID 1 = orfao de SSH caido -> kill <pid>
```

Se tudo ja estiver parado e o dockerd <5%, PARE AQUI (nada a fazer; registre o snapshot).

### H2. Neutralizar os quebrados do omnichannel-mvp (exceto caddy)

```bash
docker ps -a --filter label=com.docker.compose.project=omnichannel-mvp --format '{{.Names}}' \
  | grep -v caddy \
  | while read c; do docker update --restart=no "$c" && docker stop "$c"; done
```

`--restart=no` ANTES do stop e' o que impede a ressurreicao num reboot da VPS.
NUNCA rodar `docker compose down` em /opt/omnichannel — removeria o caddy junto.

### H3. Neutralizar omni-crow e omni-staging (com guardas)

```bash
# guarda 1: crow-web NAO pode estar na lista (projeto compose diferente):
docker inspect crow-web -f '{{index .Config.Labels "com.docker.compose.project"}}'
# guarda 2: conferir se omni-staging nao e' o staging legitimo:
docker ps -a --filter label=com.docker.compose.project=omni-staging --format '{{.Names}}' | head -1 \
  | xargs -r docker inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}'
# se retornar /home/deploy/lista-atendimento-staging: e' o staging real -> so parar, NAO remover depois.

for p in omni-crow omni-staging; do
  docker ps -a --filter label=com.docker.compose.project=$p --format '{{.Names}}' \
    | while read c; do docker update --restart=no "$c" && docker stop "$c"; done
done
```

Se o filtro por label voltar vazio, listar por prefixo e revisar A OLHO antes de parar:
`docker ps -a --format '{{.Names}}' | grep -E '^omni-(crow|staging)-'`.

### H4. Validacao (criterio de aceite do AC-02)

```bash
curl -I https://omni.crowvisuals.com.br/healthz   # HTTP 200 = painel no ar
curl -I https://omni.crowvisuals.com.br           # 200
curl -I https://crowvisuals.com.br                # 200 (crow-web intacto)
docker ps --format '{{.Names}}' | grep -E 'caddy|^crow-web$|^listaatendimento-'  # todos up
docker ps --format '{{.Names}}\t{{.Status}}' | grep -i restarting                # vazio
top -bn1 | head -5 && ps -C dockerd -o pcpu                                      # load <1, dockerd <5%
```

### H5. Rollback (tudo reversivel; volumes intactos)

```bash
docker start <container> && docker update --restart=unless-stopped <container>
```

Se o painel cair (improvavel — nada dele foi tocado): confirmar o caddy com
`docker ps | grep caddy`; ultimo recurso `docker restart omnichannel-mvp-caddy-1`
(lembrar do gotcha de inode do Caddyfile, secao do proxy acima). O snapshot de H1
(`policies-*.txt`) diz o restart-policy original de cada container.

### H6. Remocao definitiva (OPCIONAL, so apos >=7 dias estavel)

```bash
docker ps -a --filter status=exited --filter label=com.docker.compose.project=omni-crow -q | xargs -r docker rm
# omni-staging: SO se a guarda 2 do H3 confirmou que NAO e' o staging real
docker ps -a --filter status=exited --filter label=com.docker.compose.project=omni-staging -q | xargs -r docker rm
```

NUNCA: `docker volume rm`, `docker compose down -v`, prune de imagem sem revisar.
Futuro (fora deste runbook): **IN-01** — migrar o roteamento do painel para um Caddy
proprio da stack listaatendimento, eliminando a dependencia do caddy alheio.

---
```

FIM DO BLOCO A INSERIR>>>

### 3.2 Decisões já tomadas (não reabrir)

- Seleção de alvo por **label do compose** (`com.docker.compose.project=...`), nunca por
  prefixo de nome às cegas — evita pegar `crow-web` por engano; fallback por prefixo só
  com revisão manual da lista.
- `docker update --restart=no` **antes** do `docker stop` (senão o reboot ressuscita).
- Parar ≠ remover: remoção de containers é fase opcional H6 (≥7 dias), volumes nunca.
- O guard do staging usa o label `com.docker.compose.project.working_dir` para distinguir
  resíduo antigo do staging legítimo de `/home/deploy/lista-atendimento-staging`.
- Diagnóstico inclui `ss -xp`/`ps` por `docker logs` órfãos — foi a causa raiz real do
  incidente de CPU (não os crash-loops sozinhos).
- Nenhum script `.sh`/`.ps1` novo no repo — runbook é doc, execução é humana.

## 4. Critérios de aceite

Da mudança no repo (verificáveis pelo implementador):

1. `docs/DEPLOY_VPS.md` contém a seção `## Higiene da VPS (stacks orfas) — runbook manual`
   inserida antes de `## Deploy via GitHub Actions`, com o conteúdo do bloco da §3.1.
2. O arquivo permanece com ≤450 linhas (`(Get-Content docs/DEPLOY_VPS.md | Measure-Object -Line).Lines`).
3. Nenhum outro arquivo do repo alterado; nenhum comando executado contra a VPS.
4. A seção preserva intactas as instruções existentes do Caddy (linhas 145-186 atuais) —
   nada removido/reescrito, seção puramente aditiva.

Da execução na VPS (verificáveis pelo usuário, fase H4 do runbook):

5. `curl -I https://omni.crowvisuals.com.br/healthz` → 200 durante e após a limpeza.
6. `docker ps` sem nenhum container `Restarting`; `omnichannel-mvp-caddy-1`, `crow-web` e
   `listaatendimento-*` todos `Up`.
7. `ps -C dockerd -o pcpu` < 5% e load average < 1.
8. Containers órfãos com `RestartPolicy=no` (não voltam num reboot).

## 5. Validação

Pelo implementador (local, sem VPS):

```powershell
# a seção existe e está no lugar certo (antes do bloco GitHub Actions):
Select-String -Path docs/DEPLOY_VPS.md -Pattern 'Higiene da VPS','Deploy via GitHub Actions' | Select-Object LineNumber, Line
# teto de linhas:
(Get-Content docs/DEPLOY_VPS.md | Measure-Object -Line).Lines   # esperado: <= 450
```

Pelo usuário (na VPS): executar H1→H4 do runbook; H4 é a validação. O comando de smoke
já canonizado no doc (`curl -I https://omni.crowvisuals.com.br/healthz`) é o gate — se
der ≠200 em qualquer ponto, aplicar H5 (rollback) imediatamente.

## 6. Notas de Deploy

- **Nenhuma** migration, env var, Dockerfile ou dependência nova.
- Nenhum rebuild (`docker compose up -d --build api` NÃO se aplica — zero mudança em `back/`).
- A execução do runbook em si é operação de infra na VPS, feita pelo usuário, fora do
  pipeline de deploy; não bloqueia nem é bloqueada por nenhum deploy.

## 7. Regras de execução (o implementador DEVE obedecer)

- **NENHUM comando git** (sessão multi-agente — só o usuário roda git).
- **NENHUM acesso à VPS** (ssh/scp/curl contra 85.31.62.33 ou omni.crowvisuals.com.br):
  este AC é advisory; quem executa o runbook é o usuário.
- NÃO rodar npm/build/generate; não há mudança em `back/` (logo, sem rebuild da api) nem
  em `web/`.
- Máx 450 linhas por arquivo — vale para o `DEPLOY_VPS.md` editado (critério de aceite 2).
- Não remover funcionalidade/conteúdo existente do doc; a seção é aditiva.
- Zero mock/legado novo; nada a registrar em `docs/LEGADO.md` (não toca código).
- AGENT.md: nenhum módulo de código tocado → nenhum AGENT.md a atualizar.
- Idioma do doc: manter o padrão sem acentos já usado em `DEPLOY_VPS.md`.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `docs/specs/ac-fixes-2026-07/AC-02-stacks-orfas-vps.md` | esta spec (já criada) |
| `docs/DEPLOY_VPS.md` | editar — inserir seção "Higiene da VPS (stacks orfas)" antes de "## Deploy via GitHub Actions" |

Dependências: nenhuma (AC-02 é independente dos demais ACs).
Conflitos potenciais: AC-05 (backup agendado) e AC-11 (limites de memória/healthchecks)
também devem documentar procedimento em `docs/DEPLOY_VPS.md` — se rodarem em paralelo,
coordenar a ordem das edições nesse arquivo (esta seção entra antes de
"## Deploy via GitHub Actions"; as outras devem escolher âncoras distintas).
