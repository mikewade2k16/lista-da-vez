# Evolution API — guia de subida local (módulo omnichannel / F4)

Guia operacional para subir o provider **Evolution** local, gerar a API key e parear um
número de WhatsApp de teste. É infra da **F4** do canônico
([`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md) §9.2-F4, §13).

> **Contexto.** A Evolution é o **1º adapter real** do canal (decisão **D-A**, multi-provider).
> O provider `mock` roda **sem** este container — use-o para testar F5/F6/F8 sem número real.
> Este guia é só para quando você quer conectar um número de verdade.

---

## 0. O que sobe (e por que é isolado)

O `--profile omnichannel` sobe **dois** serviços novos, fora do `up` normal do dev:

| Serviço | Papel | Porta |
|---|---|---|
| `evolution` | Evolution API (Baileys). A api Go fala com ele por `http://evolution:8080` (rede interna). | `127.0.0.1:8085` no host — **só troubleshooting**; o painel lê o QR pela api Go |
| `evolution-db` | Postgres **dedicado** da Evolution (Prisma migra no boot). Isolado do postgres do Omni. | interna apenas (sem porta no host) |

**Por que um banco dedicado:** o módulo omnichannel não compartilha schema com ninguém
(canônico §4). A Evolution guarda instâncias/sessões/mensagens no banco **dela**, nunca no
`messaging.*` nem no schema do Omni.

**Nenhuma porta pública.** A porta `8085` está fixada em `127.0.0.1` — só a máquina local
alcança. Em produção, quem precisa de rota pública é o **webhook inbound**
(`/v1/webhooks/omnichannel/evolution/{slug}`), roteado pelo Caddy até a **api**, não até a
Evolution (ver §6).

---

## 1. Configurar o `.env` (antes de subir)

No `.env` da raiz (copie de `.env.docker.example` se ainda não tem). **Não commite o `.env`
real.**

```bash
# 1) Chave de cifragem de segredos do módulo (AES-256, 32 bytes em base64).
#    OBRIGATÓRIA a partir da F3.5 — SEM ela a api NÃO SOBE (fail-fast no boot).
openssl rand -base64 32
# cole o resultado em OMNI_SECRETS_KEY=

# 2) Chave mestra da Evolution (header `apikey`). O MESMO valor vai em
#    EVOLUTION_API_KEY (lido pela api Go como fallback E pelo serviço evolution).
openssl rand -hex 24
# cole o resultado em EVOLUTION_API_KEY=
```

Trecho final do `.env`:

```dotenv
OMNI_SECRETS_KEY=<saída do rand -base64 32>
EVOLUTION_BASE_URL=http://evolution:8080
EVOLUTION_API_KEY=<saída do rand -hex 24>
WEBHOOK_RECEIVER_BASE_URL=http://api:8080
EVOLUTION_DB_PASSWORD=<uma senha forte>
EVOLUTION_PORT=8085
EVOLUTION_LOG_LEVEL=ERROR
```

> `OMNI_SECRETS_KEY` **não tem default** de propósito (canônico §13-item 2). Se ficar vazia,
> a api falha no boot com mensagem nomeando a env — é o comportamento correto, não um bug.
> **Perder essa chave = perder os segredos cifrados** (sem recuperação). Guarde-a.

---

## 2. Subir a Evolution

Sobe `evolution` **e** `evolution-db` (o `depends_on` puxa o banco):

```bash
docker compose --profile omnichannel up -d evolution
```

Conferir que ficaram saudáveis (pode levar ~40s no 1º boot — a Evolution roda as migrations
Prisma dela):

```bash
docker compose --profile omnichannel ps evolution evolution-db
# STATUS deve ser "Up (healthy)" nos dois

# Sanidade do HTTP da Evolution (deve responder JSON com versão):
curl -s http://127.0.0.1:8085/ | head -c 300
```

Se `evolution` ficar em `unhealthy`/reiniciando, ver o log (sem payload cru — `LOG_LEVEL=ERROR`):

```bash
docker compose --profile omnichannel logs --tail=50 evolution
docker compose --profile omnichannel logs --tail=30 evolution-db
```

Causa mais comum no 1º boot: o `evolution-db` ainda não terminou de migrar. Aguarde o
healthcheck do banco e a Evolution reconecta sozinha.

---

## 3. Rebuild da api (se a F3.5/F4 já estiverem no código)

A api só passa a **usar** `OMNI_SECRETS_KEY`/`EVOLUTION_*` depois que o wiring (F3.5) e o
adapter (F4) estiverem no `back/`. Quando estiverem, o **orquestrador** roda (um build por vez):

```bash
docker compose up -d --build api
```

> Se a fase incluiu **migration nova**, é `docker compose build --no-cache api` antes do `up`
> (migrations são `embed.FS`; o cache da camada `go build` pode não re-embutir o `.sql`).
> Ver canônico §13.

---

## 4. Parear um número de teste (o que o DONO faz)

O pareamento é **pelo painel**, não pelo dashboard da Evolution. A api Go cria a instância,
manda a Evolution conectar, normaliza o QR e o painel exibe.

1. Logar no painel e abrir **`/omnichannel`**.
2. Ir em **configuração de números/instâncias** (tela da F10) e **cadastrar uma instância**
   com `provider = evolution`. `account_id` vem da conta ativa — nunca do formulário.
3. Clicar **Conectar**. O painel mostra o **QR code** (a api busca da Evolution e normaliza
   para data URL).
4. No celular de teste: **WhatsApp → Aparelhos conectados → Conectar aparelho** e escanear o
   QR. O QR expira em ~120s; se sumir, clicar Conectar de novo gera outro.
5. O status vira **conectado** e o número resolve. A partir daí, mensagens recebidas caem em
   `messaging.messages`.

> **Um número, uma instância (por conta).** Tentar cadastrar/conectar o **mesmo** número numa
> 2ª instância da mesma conta retorna **409** acionável, nomeando a instância que já o usa
> (garantido pelo índice único `(account_id, phone_number)` — C6 da OMNI-F4).

### Verificar que a mensagem chegou

```bash
docker compose exec postgres psql -U omni -d omni -c \
  "select id, content, created_at from messaging.messages order by created_at desc limit 1;"
```

`created_at` deve ser o **horário do provider** (não o do insert). Payload cru **nunca**
aparece no log da api:

```bash
docker compose logs api | grep -i "<um-trecho-da-mensagem>"   # deve vir VAZIO
```

---

## 5. Derrubar / limpar

```bash
# parar só a Evolution (mantém os volumes/sessão):
docker compose --profile omnichannel stop evolution evolution-db

# apagar tudo, INCLUSIVE a sessão pareada (novo QR na próxima):
docker compose --profile omnichannel down
docker volume rm omni_evolution_instances omni_evolution_db_data
# (o prefixo do volume segue COMPOSE_PROJECT_NAME — default "omni")
```

---

## 6. Produção (nota, não passo-a-passo)

- **Rota pública é do webhook, não da Evolution.** O Caddy precisa rotear
  `/v1/webhooks/*` até a **api**. Armadilha já registrada: `cat >` no Caddyfile **não pega**
  no inode do bind-mount — depois de editar, `docker restart` do container do Caddy (reload
  não basta). Ver canônico §13-item 6.
- **`WEBHOOK_RECEIVER_BASE_URL`** em prod = domínio público da api (o que a Evolution chama
  de volta).
- **Backup:** incluir o volume `evolution_instances` (sessão pareada) e o `evolution_db_data`.
- **Credencial real é por instância, cifrada** (`platform/secretbox`). `EVOLUTION_API_KEY` do
  env é só **fallback de ambiente** — a fonte é a config da instância no banco (canônico §13).

---

## Referências

- Canônico: [`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md) §4, §10, §13
- Spec da fase: [`specs/OMNI-F4.md`](specs/OMNI-F4.md)
- Proteções do webhook / ciclo de sessão: [`SPECS_PORT_OMNICHANNEL.md`](SPECS_PORT_OMNICHANNEL.md) F3
- Compose: serviços `evolution` / `evolution-db` (profile `omnichannel`); envs em `.env.docker.example`
</content>
</invoke>
