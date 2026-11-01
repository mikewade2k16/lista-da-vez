# Deploy

Leia ao fazer deploy, alterar script/workflow de deploy, ou adicionar variável de ambiente. Os detalhes de infra abaixo são padrões comuns — adapte aos do seu projeto.

## Build fora do servidor de produção (registry)
Publicar as imagens num registry e o servidor só faz `pull` + `up -d --no-build` — nunca builda. Dois caminhos para a mesma esteira de imagens: (1) rápido — build na máquina local e push (dia a dia); (2) completo/rastreável — build no CI com gate de testes e tag por commit (release formal).

- **Por quê:** builds pesados (ex.: o build do Nuxt pede ~4GB de heap) sobrecarregam um servidor pequeno com produção no ar (OOM). `push`/`pull` é incremental (só camadas que mudaram) — resolve o "deploy rápido" sem buildar no servidor.
- Nunca reintroduzir `up -d --build` no servidor de produção como caminho padrão.

## Variável de ambiente nova precisa de DUAS adições
Adicionar tanto no arquivo de env do ambiente (`.env.production` no servidor) QUANTO na seção `environment` do compose. Sem a segunda, o container não recebe a variável (o compose não propaga o arquivo `.env` inteiro automaticamente). Toda variável que afeta network (`*_PORT`, `*_URL`, `*_BASE`) deve viver em arquivo versionado por um `.env.example`, nunca em shell export — qualquer rebuild que crie um shell novo perde a config.

## Reverse proxy: restart, não reload, após mudar upstream
Alguns proxies (ex.: Caddy) mantêm cache do upstream antigo no reload. Para garantir, fazer `restart` do container do proxy. Sintoma clássico de não fazer isso: o site continua mostrando a versão antiga após o deploy.

## Banco certo antes de migrar
Antes de rodar qualquer migrate local, verificar a porta/URL do banco (`echo $DATABASE_URL`). Caso real: duas instâncias de Postgres na mesma máquina em portas diferentes; o `.env` apontava para a errada e as migrations foram aplicadas no banco errado.
