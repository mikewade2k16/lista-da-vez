# obsagent — agente de host da observabilidade

> Backlog e estado da observabilidade (o que foi feito e o que falta): `observability/AGENT.md`.

Contrato: `observability/SPEC.md`, secao "Agente de host". Binario: `back/cmd/obs-agent`
(servico `obs-agent` nos Composes, mesma imagem da API, `command: ["obs-agent"]`).

## O que faz
A cada `OBS_AGENT_INTERVAL` (padrao 30s, minimo 5s) le o Docker e o disco e grava, em
um unico batch, uma linha por container em `obs.container_samples` e uma linha
`source='agent'` em `obs.system_samples` (`{disk:[...], docker:{containers,running}}`).

## TLS (task 1.5)
`OBS_AGENT_TLS_HOSTS` (`host[:porta]`, virgula; porta padrao 443; vazio = nao checa).
A cada 6 h (cache entre ciclos, `tls.go`) faz `tls.Dial` com timeout de 5 s e
`ServerName`, le o `NotAfter` do certificado folha e grava na amostra `source='agent'`
`"tls": [{host, expiresAt, daysLeft, error}]`. Falha (DNS, timeout, certificado
invalido) vira `error` daquele host e nao derruba o ciclo. A regra `tls_expiring` fica
em `modules/observability`. No Compose de producao o padrao lista os dominios publicos
do Caddy (omni, n8n, waha e cardapios); no dev e vazio.

## Arquivos
- `docker.go`: cliente HTTP sobre socket unix, somente `GET` (`/containers/json?all=1`,
  `/containers/{id}/json`, `/containers/{id}/stats?stream=false&one-shot=true`).
  `DOCKER_HOST` so aceita `unix://`.
- `calc.go`: CPU % (formula do `docker stats`), memoria sem cache de pagina, disco
  (`statfs`, Bavail), parse de `OBS_AGENT_DISK_PATHS`.
- `collect.go`: coleta; erro em um container vira log e campos nulos. O one-shot nao
  traz `precpu_stats` confiavel, entao a CPU usa a leitura do ciclo anterior (primeiro
  ciclo: `cpu_pct` nulo).
- `store.go`: insercao em lote (`pgx.Batch`). `agent.go`: config, laco, `recover`, pool.

## Ambiente
`OBSERVABILITY_ENABLED=false` = ocioso sem gravar. Banco: `DATABASE_APP_URL` (role
`omni_app`, least-privilege) com fallback `DATABASE_URL`. `OBS_AGENT_HOST` (padrao
hostname). `OBS_AGENT_DISK_PATHS`: `caminho` ou `caminho=rotulo`, separados por virgula
(Compose usa `/host=/`: o disco do host montado `:ro` em `/host`, gravado como `/`).

## Seguranca
Montar `docker.sock` da acesso equivalente a root no host, mesmo com `:ro` (o `:ro` NAO
impede chamadas de escrita na API). Mitigacao: o codigo so emite GET, sem dependencia
de SDK, sem portas, sem rede do proxy, imagem sem segredos alem da URL do banco, limites
de CPU/memoria pequenos. Nao adicionar chamadas nao-GET. Logs nao incluem env nem labels.

## Teste manual sem gravar
`go run ./cmd/obs-agent -once -dry-run` imprime o snapshot em JSON (nao toca o banco).
Testes: `go test ./internal/obsagent/`.

## restart_policy e exit_code
Cada amostra de container grava `HostConfig.RestartPolicy.Name` em `restart_policy` e `State.ExitCode` em `exit_code` (só quando não está running). A API usa a policy para distinguir container parado de propósito (`no`) de queda real.
