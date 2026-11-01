# Ambiente no Mac Apple Silicon

## Stack completa local

Com Docker Desktop aberto e `.env` local configurado (usar `.env.docker.example`
como referência, mantendo as chaves do banco restaurado):

```bash
npm run dev:mac:up
npm run dev:mac:ps
npm run dev:mac:stop
```

O comando sobe API, web, proxy local, PostgreSQL, Redis, n8n, WAHA, Whisper,
Evolution e seu banco, além do runner de compatibilidade Meta Ads. Usa os profiles
explicitamente; `docker compose up -d` sozinho não ativa serviços com profiles.
O runner Meta é legado de diagnóstico; o Assistente 360 usa a API Go.
WAHA usa emulação AMD64 no Apple Silicon.

A rede `omni_local_services` permite saída para WhatsApp, modelos e provedores.
As portas dos auxiliares são locais (`127.0.0.1`): n8n 5680, WAHA 3010,
Whisper 8010, Evolution 8085, Redis 6380 e runner 8765, salvo override no `.env`.
Painel: http://localhost:3003; API: http://localhost:8080/healthz.
Volumes persistem durante recriações; nunca apagar volumes para corrigir rede.

`crow-nuxt` depende do repositório irmão `../crow-nuxt`, não incluído neste projeto.
Fica no profile `bio` do override Mac; só iniciá-lo explicitamente com esse
repositório disponível. A URL pública da bio pode apontar para o site publicado.
Containers temporários de build/check/release que terminaram com código zero
não são serviços e devem permanecer encerrados.

Container saudável confirma disponibilidade do serviço. WhatsApp ainda depende
do pareamento; IA, e-mail, Meta e workflows dependem das credenciais/configurações
do tenant e do ambiente. Não criar seeds ou ativar workflows para simular sucesso.

Verificação em 2026-11-01: 11 containers ativos; API, web, PostgreSQL, Redis,
n8n, WAHA, Whisper, Evolution, banco Evolution e runner Meta saudáveis; proxy
local respondendo. Painel e `/healthz` retornaram HTTP 200. As chamadas da API
para n8n, WAHA, Whisper e Evolution também retornaram 200.
O n8n local não possui workflows, WAHA não possui sessões e Evolution não possui
instâncias. O runner legado Meta não possui autenticação Claude. Restaurar um
backup autorizado com credenciais ou configurar essas integrações antes de
considerar os fluxos ponta a ponta prontos. Não foram alterados workflows,
credenciais, dados de usuários ou código do Finance nesta intervenção.

Configurações de agentes: [guia compartilhado](agents/CONFIGURACAO.md).

Setup local (2026-10-29), sem sudo e sem Homebrew:

```bash
ruby scripts/dev/setup-mac-tools.rb
```

O instalador baixa os arquivos oficiais, verifica SHA-256 e instala:

- Node 24.11.1 e npm, alinhados ao Dockerfile do projeto;
- PowerShell 7.6.9 ARM64, usado pelos scripts de deploy;
- prompt com cores, diretorio e branch Git para zsh e bash.

Executaveis em `~/.local/bin`; distribuicoes em `~/.local/share/node` e
`~/.local/share/powershell`. O shell padrao deste Mac e zsh; nao e necessario troca-lo.
Configuracoes existentes sao preservadas e copiadas para
`~/.local/state/omni-mac-setup/<data>/` antes de adicionar o carregador.
O prompt e o PATH ficam em `~/.config/crow-shell/`.

Abra um novo terminal ou, no zsh atual:

```bash
source ~/.zshrc
node --version
npm --version
pwsh --version
```

O prompt mostra a branch atual (exemplo: `main`) dentro de um repositorio Git.
Cores exigem um terminal interativo com suporte ANSI; execucoes automatizadas com
`TERM=dumb` usam o prompt simples.

O Node do host permite executar os atalhos npm e ferramentas de orquestracao.
O desenvolvimento do app continua Docker-first; nao e preciso instalar as dependencias
do frontend no host para usar os atalhos de deploy.

## Deploy

Os atalhos usam `pwsh` no Mac/Linux e `powershell.exe` no Windows.
Veja [DEPLOY_VPS.md](DEPLOY_VPS.md). Consultar ajuda nao faz build nem publica:

```bash
npm run deploy -- --help
```

Para publicar, o Docker Desktop precisa estar aberto, o GHCR autenticado com permissao
de publicar imagens e a chave/usuario SSH autorizados configurados. Se a chave padrao
nao existir, informe `-KeyPath`; o deploy rapido verifica isso antes de qualquer build.
As imagens usam `linux/amd64` por padrao para a arquitetura da VPS.
Um pedido de instalacao/configuracao do ambiente nao executa deploy nem build de validacao.

O launcher npm le `~/.config/omni/deploy.json` para configurar o acesso deste computador:

```json
{
  "sshUser": "USUARIO_AUTORIZADO",
  "sshKeyPath": "~/.ssh/omni_vps_mac"
}
```

O arquivo e local e a chave privada permanece em `~/.ssh/`. Nao colocar chaves no
repositorio. Precedencia: argumentos `-User`/`-KeyPath`, variaveis `OMNI_DEPLOY_USER`/
`OMNI_DEPLOY_KEY_PATH`, configuracao local, defaults dos scripts.
Assim `npm run deploy` funciona sem repetir o caminho da chave em cada comando,
inclusive nos scripts aninhados de staging/promocao. Ajuda nao le nem usa credenciais.
